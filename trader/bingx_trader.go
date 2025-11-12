package trader

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	bingxMainnetBaseURL = "https://open-api.bingx.com"
	bingxTestnetBaseURL = "https://open-api-vst.bingx.com"
	defaultRecvWindow   = 5000
)

// BingxTrader 实现 Trader 接口，支持 BingX 永续合约
type BingxTrader struct {
	apiKey    string
	secretKey string
	client    *http.Client
	baseURL   string

	symbolCache map[string]bingxSymbolInfo
	cacheMu     sync.RWMutex
}

type bingxResponse struct {
	Code     int             `json:"code"`
	Msg      string          `json:"msg"`
	DebugMsg string          `json:"debugMsg"`
	Data     json.RawMessage `json:"data"`
}

type bingxSymbolInfo struct {
	Symbol            string
	QuantityPrecision int
	PricePrecision    int
	TradeMinQuantity  float64
	TradeMinUSDT      float64
	QuantityStep      float64
}

type stringNumber string

func (sn *stringNumber) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "" {
		*sn = ""
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	*sn = stringNumber(s)
	return nil
}

func (sn stringNumber) Float64() float64 {
	if sn == "" {
		return 0
	}
	f, err := strconv.ParseFloat(string(sn), 64)
	if err != nil {
		return 0
	}
	return f
}

// NewBingxTrader 创建 BingX 交易器实例
func NewBingxTrader(apiKey, secretKey string, testnet bool) (*BingxTrader, error) {
	apiKey = strings.TrimSpace(apiKey)
	secretKey = strings.TrimSpace(secretKey)
	if apiKey == "" || secretKey == "" {
		return nil, fmt.Errorf("BingX API Key 和 Secret Key 不能为空")
	}

	baseURL := bingxMainnetBaseURL
	if testnet {
		baseURL = bingxTestnetBaseURL
	}

	return &BingxTrader{
		apiKey:      apiKey,
		secretKey:   secretKey,
		baseURL:     baseURL,
		client:      &http.Client{Timeout: 15 * time.Second},
		symbolCache: make(map[string]bingxSymbolInfo),
		cacheMu:     sync.RWMutex{},
	}, nil
}

// GetBalance 获取账户余额
func (t *BingxTrader) GetBalance() (map[string]interface{}, error) {
	data, err := t.signedRequest(context.Background(), http.MethodGet, "/openApi/swap/v1/user/marginAssets", nil)
	if err != nil {
		return nil, err
	}

	var assets []struct {
		Currency             string `json:"currency"`
		TotalAmount          string `json:"totalAmount"`
		AvailableTransfer    string `json:"availableTransfer"`
		LatestMortgageAmount string `json:"latestMortgageAmount"`
	}

	if err := json.Unmarshal(data, &assets); err != nil {
		return nil, fmt.Errorf("解析账户余额失败: %w", err)
	}

	total := 0.0
	available := 0.0
	for _, asset := range assets {
		amount, _ := strconv.ParseFloat(asset.TotalAmount, 64)
		free, _ := strconv.ParseFloat(asset.AvailableTransfer, 64)
		if strings.EqualFold(asset.Currency, "USDT") {
			total = amount
			available = free
			break
		}
	}

	// 如果未找到USDT资产，则回退到累加模式
	if total == 0 && len(assets) > 0 {
		for _, asset := range assets {
			amount, _ := strconv.ParseFloat(asset.TotalAmount, 64)
			free, _ := strconv.ParseFloat(asset.AvailableTransfer, 64)
			total += amount
			available += free
		}
	}

	result := map[string]interface{}{
		"totalWalletBalance": total,
		"availableBalance":   available,
		"assets":             assets,
	}
	return result, nil
}

// GetPositions 获取账户持仓
func (t *BingxTrader) GetPositions() ([]map[string]interface{}, error) {
	data, err := t.signedRequest(context.Background(), http.MethodGet, "/openApi/swap/v2/user/positions", nil)
	if err != nil {
		return nil, err
	}

	var rawPositions []struct {
		Symbol           string       `json:"symbol"`
		PositionAmt      string       `json:"positionAmt"`
		PositionSide     string       `json:"positionSide"`
		AvgPrice         string       `json:"avgPrice"`
		UnrealizedProfit string       `json:"unrealizedProfit"`
		Leverage         int          `json:"leverage"`
		LiquidationPrice stringNumber `json:"liquidationPrice"`
	}

	if err := json.Unmarshal(data, &rawPositions); err != nil {
		return nil, fmt.Errorf("解析持仓信息失败: %w", err)
	}

	var positions []map[string]interface{}
	for _, pos := range rawPositions {
		positionAmt, _ := strconv.ParseFloat(pos.PositionAmt, 64)
		if positionAmt == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.AvgPrice, 64)
		unrealized, _ := strconv.ParseFloat(pos.UnrealizedProfit, 64)
		liqPrice := pos.LiquidationPrice.Float64()

		internalSymbol := normalizeInternalSymbol(pos.Symbol)
		markPrice, err := t.GetMarketPrice(internalSymbol)
		if err != nil {
			return nil, err
		}

		side := strings.ToUpper(pos.PositionSide)
		if side != "LONG" && side != "SHORT" {
			if positionAmt > 0 {
				side = "LONG"
			} else {
				side = "SHORT"
			}
		}

		position := map[string]interface{}{
			"symbol":           internalSymbol,
			"positionAmt":      positionAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unrealized,
			"leverage":         float64(pos.Leverage),
			"positionSide":     side,
			"liquidationPrice": liqPrice,
		}
		position["side"] = side
		positions = append(positions, position)
	}

	return positions, nil
}

// OpenLong 开多仓
func (t *BingxTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if err := t.CancelAllOrders(symbol); err != nil {
		logIfError("BingX 取消旧委托单失败", err)
	}

	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	return t.createMarketOrder(symbol, quantity, "BUY", "LONG", false)
}

// OpenShort 开空仓
func (t *BingxTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if err := t.CancelAllOrders(symbol); err != nil {
		logIfError("BingX 取消旧委托单失败", err)
	}

	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	return t.createMarketOrder(symbol, quantity, "SELL", "SHORT", false)
}

// CloseLong 平多仓
func (t *BingxTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.createMarketOrder(symbol, quantity, "SELL", "LONG", true)
}

// CloseShort 平空仓
func (t *BingxTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	return t.createMarketOrder(symbol, quantity, "BUY", "SHORT", true)
}

// SetLeverage 设置杠杆（双向持仓需分别设置 LONG/SHORT）
func (t *BingxTrader) SetLeverage(symbol string, leverage int) error {
	if leverage < 1 {
		leverage = 1
	}
	if leverage > 125 {
		leverage = 125
	}

	for _, side := range []string{"LONG", "SHORT"} {
		params := map[string]string{
			"symbol":   toBingxSymbol(symbol),
			"side":     side,
			"leverage": strconv.Itoa(leverage),
		}
		if _, err := t.signedRequest(context.Background(), http.MethodPost, "/openApi/swap/v2/trade/leverage", params); err != nil {
			return fmt.Errorf("设置杠杆失败(%s): %w", side, err)
		}
	}
	return nil
}

// SetMarginMode 设置保证金模式
func (t *BingxTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	mode := "ISOLATED"
	if isCrossMargin {
		mode = "CROSSED"
	}

	params := map[string]string{
		"symbol":     toBingxSymbol(symbol),
		"marginType": mode,
	}
	if _, err := t.signedRequest(context.Background(), http.MethodPost, "/openApi/swap/v2/trade/marginType", params); err != nil {
		return fmt.Errorf("设置保证金模式失败: %w", err)
	}
	return nil
}

// GetMarketPrice 获取最新价格
func (t *BingxTrader) GetMarketPrice(symbol string) (float64, error) {
	params := map[string]string{
		"symbol": toBingxSymbol(symbol),
	}
	data, err := t.publicRequest(context.Background(), http.MethodGet, "/openApi/swap/v2/quote/ticker", params)
	if err != nil {
		return 0, err
	}

	var ticker struct {
		LastPrice string `json:"lastPrice"`
	}
	if err := json.Unmarshal(data, &ticker); err != nil {
		return 0, fmt.Errorf("解析价格信息失败: %w", err)
	}

	price, err := strconv.ParseFloat(ticker.LastPrice, 64)
	if err != nil {
		return 0, fmt.Errorf("无效的价格: %s", ticker.LastPrice)
	}
	return price, nil
}

// SetStopLoss 设置止损单
func (t *BingxTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	return t.placeStopOrder(symbol, positionSide, quantity, stopPrice, "STOP_MARKET")
}

// SetTakeProfit 设置止盈单
func (t *BingxTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	return t.placeStopOrder(symbol, positionSide, quantity, takeProfitPrice, "TAKE_PROFIT_MARKET")
}

// CancelAllOrders 取消所有类型的挂单
func (t *BingxTrader) CancelAllOrders(symbol string) error {
	return t.cancelOrdersByType(symbol, "")
}

// CancelStopLossOrders 取消止损单
func (t *BingxTrader) CancelStopLossOrders(symbol string) error {
	return t.cancelOrdersByType(symbol, "STOP_MARKET")
}

// CancelTakeProfitOrders 取消止盈单
func (t *BingxTrader) CancelTakeProfitOrders(symbol string) error {
	return t.cancelOrdersByType(symbol, "TAKE_PROFIT_MARKET")
}

// CancelStopOrders 取消止盈/止损单
func (t *BingxTrader) CancelStopOrders(symbol string) error {
	if err := t.CancelStopLossOrders(symbol); err != nil {
		return err
	}
	return t.CancelTakeProfitOrders(symbol)
}

// FormatQuantity 根据交易对精度格式化数量
func (t *BingxTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	info, err := t.getSymbolInfo(symbol)
	if err != nil {
		return "", err
	}

	qty := quantity
	if info.QuantityStep > 0 {
		qty = roundToTickSize(quantity, info.QuantityStep)
	}

	if qty <= 0 {
		return "", fmt.Errorf("数量 %.8f 无效，无法下单", quantity)
	}

	precision := info.QuantityPrecision
	if precision == 0 {
		if info.QuantityStep > 0 && info.QuantityStep < 1 {
			precision = decimalsFromStep(info.QuantityStep)
		} else if qty < 1 {
			precision = decimalsFromValue(qty)
		}
		if precision == 0 {
			precision = 6
		}
	}

	minQty := info.TradeMinQuantity
	if minQty == 0 && info.QuantityStep > 0 {
		minQty = info.QuantityStep
	}
	if minQty > 0 && qty < minQty {
		return "", fmt.Errorf("数量 %.8f 小于最小下单数量 %.8f", qty, minQty)
	}

	return trimFloatString(qty, precision), nil
}

// -------------------- 私有辅助函数 --------------------

func (t *BingxTrader) createMarketOrder(symbol string, quantity float64, side, positionSide string, reduceOnly bool) (map[string]interface{}, error) {
	qtyStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	if err := t.CheckMinNotional(symbol, parseStringToFloat(qtyStr)); err != nil {
		return nil, err
	}

	params := map[string]string{
		"symbol":       toBingxSymbol(symbol),
		"side":         side,
		"positionSide": positionSide,
		"type":         "MARKET",
		"quantity":     qtyStr,
	}

	if reduceOnly {
		params["reduceOnly"] = "true"
	}

	data, err := t.signedRequest(context.Background(), http.MethodPost, "/openApi/swap/v2/trade/order", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Order map[string]interface{} `json:"order"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析订单响应失败: %w", err)
	}

	if resp.Order == nil {
		resp.Order = map[string]interface{}{
			"symbol":       symbol,
			"positionSide": positionSide,
		}
	}
	return resp.Order, nil
}

func (t *BingxTrader) placeStopOrder(symbol, positionSide string, quantity, targetPrice float64, orderType string) error {
	qtyStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	info, err := t.getSymbolInfo(symbol)
	if err != nil {
		return err
	}

	stopPrice := formatToPrecision(targetPrice, info.PricePrecision)
	if stopPrice <= 0 {
		return fmt.Errorf("无效的价格 %.8f", targetPrice)
	}

	params := map[string]string{
		"symbol":       toBingxSymbol(symbol),
		"positionSide": strings.ToUpper(positionSide),
		"type":         orderType,
		"quantity":     qtyStr,
		"stopPrice":    trimFloatString(stopPrice, info.PricePrecision),
		"workingType":  "MARK_PRICE",
		"reduceOnly":   "true",
	}

	// 止损方向与仓位相反
	if strings.EqualFold(positionSide, "LONG") {
		params["side"] = "SELL"
	} else {
		params["side"] = "BUY"
	}

	_, err = t.signedRequest(context.Background(), http.MethodPost, "/openApi/swap/v2/trade/order", params)
	if err != nil {
		return fmt.Errorf("设置%s失败: %w", orderType, err)
	}
	return nil
}

func (t *BingxTrader) cancelOrdersByType(symbol string, orderType string) error {
	params := map[string]string{
		"symbol": toBingxSymbol(symbol),
	}
	if orderType != "" {
		params["type"] = orderType
	}

	_, err := t.signedRequest(context.Background(), http.MethodDelete, "/openApi/swap/v2/trade/allOpenOrders", params)
	if err != nil {
		return fmt.Errorf("取消订单失败: %w", err)
	}
	return nil
}

// CheckMinNotional 检查最小名义金额
func (t *BingxTrader) CheckMinNotional(symbol string, quantity float64) error {
	info, err := t.getSymbolInfo(symbol)
	if err != nil {
		return err
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return fmt.Errorf("获取市价失败: %w", err)
	}

	notional := price * quantity
	minNotional := info.TradeMinUSDT
	if minNotional <= 0 {
		minNotional = 2 // BingX 默认最小 2 USDT
	}

	if notional < minNotional {
		return fmt.Errorf("订单金额 %.2f USDT 低于最小要求 %.2f USDT", notional, minNotional)
	}
	return nil
}

func (t *BingxTrader) getSymbolInfo(symbol string) (bingxSymbolInfo, error) {
	internal := strings.ToUpper(symbol)
	t.cacheMu.RLock()
	info, ok := t.symbolCache[internal]
	t.cacheMu.RUnlock()
	if ok {
		return info, nil
	}

	// 加载所有合约信息
	data, err := t.publicRequest(context.Background(), http.MethodGet, "/openApi/swap/v2/quote/contracts", nil)
	if err != nil {
		return bingxSymbolInfo{}, err
	}

	var contracts []struct {
		Symbol            string       `json:"symbol"`
		QuantityPrecision int          `json:"quantityPrecision"`
		PricePrecision    int          `json:"pricePrecision"`
		TradeMinQuantity  stringNumber `json:"tradeMinQuantity"`
		TradeMinUSDT      stringNumber `json:"tradeMinUSDT"`
	}
	if err := json.Unmarshal(data, &contracts); err != nil {
		return bingxSymbolInfo{}, fmt.Errorf("解析交易规则失败: %w", err)
	}

	cache := make(map[string]bingxSymbolInfo)
	for _, c := range contracts {
		internalSymbol := normalizeInternalSymbol(c.Symbol)
		minQty := c.TradeMinQuantity.Float64()
		minUSDT := c.TradeMinUSDT.Float64()
		cache[internalSymbol] = bingxSymbolInfo{
			Symbol:            c.Symbol,
			QuantityPrecision: c.QuantityPrecision,
			PricePrecision:    c.PricePrecision,
			TradeMinQuantity:  minQty,
			TradeMinUSDT:      minUSDT,
			QuantityStep:      minQty,
		}
	}

	t.cacheMu.Lock()
	t.symbolCache = cache
	t.cacheMu.Unlock()

	info, ok = cache[internal]
	if !ok {
		return bingxSymbolInfo{}, fmt.Errorf("未找到交易对 %s 的规则信息", symbol)
	}
	return info, nil
}

func (t *BingxTrader) signedRequest(ctx context.Context, method, path string, params map[string]string) (json.RawMessage, error) {
	values := url.Values{}
	for k, v := range params {
		if v != "" {
			values.Set(k, v)
		}
	}
	if values.Get("timestamp") == "" {
		values.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	}
	if values.Get("recvWindow") == "" {
		values.Set("recvWindow", strconv.Itoa(defaultRecvWindow))
	}

	payload := buildPayload(values)
	signature := signPayload(payload, t.secretKey)

	var fullQuery string
	if payload == "" {
		fullQuery = "signature=" + signature
	} else {
		fullQuery = payload + "&signature=" + signature
	}

	endpoint := fmt.Sprintf("%s%s?%s", t.baseURL, path, fullQuery)
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-BX-APIKEY", t.apiKey)
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result bingxResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析BingX响应失败: %w", err)
	}

	if result.Code != 0 {
		if result.DebugMsg != "" {
			return nil, fmt.Errorf("BingX API 错误[%d]: %s - %s", result.Code, result.Msg, result.DebugMsg)
		}
		return nil, fmt.Errorf("BingX API 错误[%d]: %s", result.Code, result.Msg)
	}

	return result.Data, nil
}

func (t *BingxTrader) publicRequest(ctx context.Context, method, path string, params map[string]string) (json.RawMessage, error) {
	values := url.Values{}
	for k, v := range params {
		if v != "" {
			values.Set(k, v)
		}
	}
	endpoint := t.baseURL + path
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result bingxResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析BingX响应失败: %w", err)
	}

	if result.Code != 0 {
		if result.DebugMsg != "" {
			return nil, fmt.Errorf("BingX API 错误[%d]: %s - %s", result.Code, result.Msg, result.DebugMsg)
		}
		return nil, fmt.Errorf("BingX API 错误[%d]: %s", result.Code, result.Msg)
	}

	return result.Data, nil
}

func toBingxSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.Contains(symbol, "-") {
		return symbol
	}

	quoteCurrencies := []string{"USDT", "USDC", "BUSD"}
	for _, quote := range quoteCurrencies {
		if strings.HasSuffix(symbol, quote) {
			base := strings.TrimSuffix(symbol, quote)
			if base != "" {
				return base + "-" + quote
			}
		}
	}
	if len(symbol) > 4 {
		return symbol[:len(symbol)-4] + "-" + symbol[len(symbol)-4:]
	}
	return symbol
}

func normalizeInternalSymbol(symbol string) string {
	return strings.ReplaceAll(strings.ToUpper(symbol), "-", "")
}

func buildPayload(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	first := true
	for _, key := range keys {
		for _, v := range values[key] {
			if !first {
				builder.WriteByte('&')
			} else {
				first = false
			}
			builder.WriteString(key)
			builder.WriteByte('=')
			builder.WriteString(v)
		}
	}
	return builder.String()
}

func signPayload(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func formatToPrecision(value float64, precision int) float64 {
	if precision < 0 {
		precision = 0
	}
	multiplier := math.Pow10(precision)
	return math.Floor(value*multiplier) / multiplier
}

func trimFloatString(value float64, precision int) string {
	str := strconv.FormatFloat(value, 'f', precision, 64)
	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	if str == "" {
		return "0"
	}
	return str
}

func decimalsFromStep(step float64) int {
	if step <= 0 {
		return 0
	}
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.12f", step), "0"), ".")
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		return len(s) - idx - 1
	}
	return 0
}

func decimalsFromValue(val float64) int {
	if val <= 0 {
		return 0
	}
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.12f", val), "0"), ".")
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		return len(s) - idx - 1
	}
	return 0
}

func parseStringToFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func logIfError(message string, err error) {
	if err != nil {
		log.Printf("⚠️ %s: %v", message, err)
	}
}
