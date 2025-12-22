package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FundingRateCache 资金费率缓存结构
// Binance Funding Rate 每 8 小时才更新一次，使用 1 小时缓存可显著减少 API 调用
type FundingRateCache struct {
	Rate      float64
	UpdatedAt time.Time
}

var (
	fundingRateMap sync.Map // map[string]*FundingRateCache
	frCacheTTL     = 1 * time.Hour
)

// Get 获取指定代币的市场数据
func Get(symbol string) (*Data, error) {
	var klines3m, klines4h []Kline
	var err error
	// 标准化symbol
	symbol = Normalize(symbol)
	// 获取3分钟K线数据 (最近10个)
	klines3m, err = WSMonitorCli.GetCurrentKlines(symbol, "3m") // 多获取一些用于计算
	if err != nil {
		return nil, fmt.Errorf("获取3分钟K线失败: %v", err)
	}

	// 获取4小时K线数据 (最近10个)
	klines4h, err = WSMonitorCli.GetCurrentKlines(symbol, "4h") // 多获取用于计算指标
	if err != nil {
		return nil, fmt.Errorf("获取4小时K线失败: %v", err)
	}

	// 检查数据是否为空
	if len(klines3m) == 0 {
		return nil, fmt.Errorf("3分钟K线数据为空")
	}
	if len(klines4h) == 0 {
		return nil, fmt.Errorf("4小时K线数据为空")
	}

	// 计算当前指标 (基于3分钟最新数据)
	currentPrice := klines3m[len(klines3m)-1].Close
	currentEMA20 := calculateEMA(klines3m, 20)
	currentMACD := calculateMACD(klines3m)
	currentRSI7 := calculateRSI(klines3m, 7)

	// 计算价格变化百分比
	// 1小时价格变化 = 20个3分钟K线前的价格
	priceChange1h := 0.0
	if len(klines3m) >= 21 { // 至少需要21根K线 (当前 + 20根前)
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}

	// 4小时价格变化 = 1个4小时K线前的价格
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}

	// 获取OI数据
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: 0, Average: 0}
	}

	// 获取Funding Rate
	fundingRate, _ := getFundingRate(symbol)

	// 计算日内系列数据
	intradayData := calculateIntradaySeries(klines3m)

	// 计算长期数据
	longerTermData := calculateLongerTermData(klines4h)

	// 计算Ichimoku Cloud数据 (基于4小时数据)
	ichimokuData := calculateIchimoku(klines4h)

	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
		IchimokuCloud:     ichimokuData,
	}, nil
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateMACD 计算MACD
func calculateMACD(klines []Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	// 计算12期和26期EMA
	ema12 := calculateEMA(klines, 12)
	ema26 := calculateEMA(klines, 26)

	// MACD = EMA12 - EMA26
	return ema12 - ema26
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateIntradaySeries 计算日内系列数据
func calculateIntradaySeries(klines []Kline) *IntradayData {
	data := &IntradayData{
		MidPrices:   make([]float64, 0, 10),
		EMA20Values: make([]float64, 0, 10),
		MACDValues:  make([]float64, 0, 10),
		RSI7Values:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
		Volume:      make([]float64, 0, 10),
	}

	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		data.MidPrices = append(data.MidPrices, klines[i].Close)
		data.Volume = append(data.Volume, klines[i].Volume)

		// 计算每个点的EMA20
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Values = append(data.EMA20Values, ema20)
		}

		// 计算每个点的MACD
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}

		// 计算每个点的RSI
		if i >= 7 {
			rsi7 := calculateRSI(klines[:i+1], 7)
			data.RSI7Values = append(data.RSI7Values, rsi7)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	// 计算3m ATR14
	data.ATR14 = calculateATR(klines, 14)

	// 计算Ichimoku Cloud数据 (基于3分钟数据用于短期分析)
	data.IchimokuCloud = calculateIchimoku(klines)

	return data
}

// calculateLongerTermData
func calculateLongerTermData(klines []Kline) *LongerTermData {
	data := &LongerTermData{
		MACDValues:  make([]float64, 0, 10),
		RSI14Values: make([]float64, 0, 10),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)

	// 计算ATR
	data.ATR3 = calculateATR(klines, 3)
	data.ATR14 = calculateATR(klines, 14)

	// 计算成交量
	if len(klines) > 0 {
		data.CurrentVolume = klines[len(klines)-1].Volume
		// 计算平均成交量
		sum := 0.0
		for _, k := range klines {
			sum += k.Volume
		}
		data.AverageVolume = sum / float64(len(klines))
	}

	// 计算MACD和RSI序列
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}

	for i := start; i < len(klines); i++ {
		if i >= 25 {
			macd := calculateMACD(klines[:i+1])
			data.MACDValues = append(data.MACDValues, macd)
		}
		if i >= 14 {
			rsi14 := calculateRSI(klines[:i+1], 14)
			data.RSI14Values = append(data.RSI14Values, rsi14)
		}
	}

	// 计算Ichimoku Cloud数据 (基于4小时数据用于长期分析)
	data.IchimokuCloud = calculateIchimoku(klines)

	return data
}

// getOpenInterestData 获取OI数据
func getOpenInterestData(symbol string) (*OIData, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // 近似平均值
	}, nil
}

// getFundingRate 获取资金费率（优化：使用 1 小时缓存）
func getFundingRate(symbol string) (float64, error) {
	// 检查缓存（有效期 1 小时）
	// Funding Rate 每 8 小时才更新，1 小时缓存非常合理
	if cached, ok := fundingRateMap.Load(symbol); ok {
		cache := cached.(*FundingRateCache)
		if time.Since(cache.UpdatedAt) < frCacheTTL {
			// 缓存命中，直接返回
			return cache.Rate, nil
		}
	}

	// 缓存过期或不存在，调用 API
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	apiClient := NewAPIClient()
	resp, err := apiClient.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)

	// 更新缓存
	fundingRateMap.Store(symbol, &FundingRateCache{
		Rate:      rate,
		UpdatedAt: time.Now(),
	})

	return rate, nil
}

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	// 使用动态精度格式化价格
	priceStr := formatPriceWithDynamicPrecision(data.CurrentPrice)
	sb.WriteString(fmt.Sprintf("current_price = %s, current_ema20 = %.3f, current_macd = %.3f, current_rsi (7 period) = %.3f\n\n",
		priceStr, data.CurrentEMA20, data.CurrentMACD, data.CurrentRSI7))

	sb.WriteString(fmt.Sprintf("In addition, here is the latest %s open interest and funding rate for perps:\n\n",
		data.Symbol))

	if data.OpenInterest != nil {
		// 使用动态精度格式化 OI 数据
		oiLatestStr := formatPriceWithDynamicPrecision(data.OpenInterest.Latest)
		oiAverageStr := formatPriceWithDynamicPrecision(data.OpenInterest.Average)
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %s Average: %s\n\n",
			oiLatestStr, oiAverageStr))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))

	if data.IntradaySeries != nil {
		sb.WriteString("Intraday series (3‑minute intervals, oldest → latest):\n\n")

		if len(data.IntradaySeries.MidPrices) > 0 {
			sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
		}

		if len(data.IntradaySeries.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA indicators (20‑period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
		}

		if len(data.IntradaySeries.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
		}

		if len(data.IntradaySeries.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (7‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
		}

		if len(data.IntradaySeries.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
		}

		if len(data.IntradaySeries.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
		}

		sb.WriteString(fmt.Sprintf("3m ATR (14‑period): %.3f\n\n", data.IntradaySeries.ATR14))
	}

	if data.LongerTermContext != nil {
		sb.WriteString("Longer‑term context (4‑hour timeframe):\n\n")

		sb.WriteString(fmt.Sprintf("20‑Period EMA: %.3f vs. 50‑Period EMA: %.3f\n\n",
			data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))

		sb.WriteString(fmt.Sprintf("3‑Period ATR: %.3f vs. 14‑Period ATR: %.3f\n\n",
			data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

		sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
			data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))

		if len(data.LongerTermContext.MACDValues) > 0 {
			sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
		}

		if len(data.LongerTermContext.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI indicators (14‑Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
		}
	}

	// Ichimoku Cloud 分析
	if data.IchimokuCloud != nil {
		sb.WriteString("Ichimoku Cloud Analysis:\n\n")
		
		sb.WriteString(fmt.Sprintf("Tenkan-Sen (Conversion Line): %.4f\n", data.IchimokuCloud.TenkanSen))
		sb.WriteString(fmt.Sprintf("Kijun-Sen (Base Line): %.4f\n", data.IchimokuCloud.KijunSen))
		sb.WriteString(fmt.Sprintf("Senkou Span A (Leading Span A): %.4f\n", data.IchimokuCloud.SenkouSpanA))
		sb.WriteString(fmt.Sprintf("Senkou Span B (Leading Span B): %.4f\n", data.IchimokuCloud.SenkouSpanB))
		sb.WriteString(fmt.Sprintf("Chikou Span (Lagging Span): %.4f\n\n", data.IchimokuCloud.ChikouSpan))
		
		sb.WriteString(fmt.Sprintf("Cloud Color: %s\n", data.IchimokuCloud.CloudColor))
		sb.WriteString(fmt.Sprintf("Cloud Thickness: %.4f\n", data.IchimokuCloud.CloudThickness))
		sb.WriteString(fmt.Sprintf("Price Position: %s cloud\n", data.IchimokuCloud.PricePosition))
		sb.WriteString(fmt.Sprintf("Trend Direction: %s\n\n", data.IchimokuCloud.TrendDirection))
		
		// 显示历史序列数据
		if len(data.IchimokuCloud.TenkanSenSeries) > 0 {
			sb.WriteString(fmt.Sprintf("Tenkan-Sen Series: %s\n", formatFloatSlice(data.IchimokuCloud.TenkanSenSeries)))
		}
		if len(data.IchimokuCloud.KijunSenSeries) > 0 {
			sb.WriteString(fmt.Sprintf("Kijun-Sen Series: %s\n", formatFloatSlice(data.IchimokuCloud.KijunSenSeries)))
		}
		if len(data.IchimokuCloud.SenkouSpanASeries) > 0 {
			sb.WriteString(fmt.Sprintf("Senkou Span A Series: %s\n", formatFloatSlice(data.IchimokuCloud.SenkouSpanASeries)))
		}
		if len(data.IchimokuCloud.SenkouSpanBSeries) > 0 {
			sb.WriteString(fmt.Sprintf("Senkou Span B Series: %s\n\n", formatFloatSlice(data.IchimokuCloud.SenkouSpanBSeries)))
		}
		
		// 交易信号解释
		sb.WriteString("Ichimoku Trading Signals:\n")
		if data.IchimokuCloud.TenkanSen > data.IchimokuCloud.KijunSen {
			sb.WriteString("• Tenkan above Kijun: Bullish momentum\n")
		} else if data.IchimokuCloud.TenkanSen < data.IchimokuCloud.KijunSen {
			sb.WriteString("• Tenkan below Kijun: Bearish momentum\n")
		} else {
			sb.WriteString("• Tenkan equals Kijun: Neutral momentum\n")
		}
		
		if data.IchimokuCloud.CloudColor == "bullish" {
			sb.WriteString("• Bullish Cloud: Span A above Span B\n")
		} else if data.IchimokuCloud.CloudColor == "bearish" {
			sb.WriteString("• Bearish Cloud: Span A below Span B\n")
		} else {
			sb.WriteString("• Neutral Cloud: Span A equals Span B\n")
		}
		
		switch data.IchimokuCloud.PricePosition {
		case "above":
			sb.WriteString("• Price above cloud: Strong bullish signal\n")
		case "below":
			sb.WriteString("• Price below cloud: Strong bearish signal\n")
		case "inside":
			sb.WriteString("• Price inside cloud: Consolidation/uncertainty\n")
		}
		
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatPriceWithDynamicPrecision 根据价格区间动态选择精度
// 这样可以完美支持从超低价 meme coin (< 0.0001) 到 BTC/ETH 的所有币种
func formatPriceWithDynamicPrecision(price float64) string {
	switch {
	case price < 0.0001:
		// 超低价 meme coin: 1000SATS, 1000WHY, DOGS
		// 0.00002070 → "0.00002070" (8位小数)
		return fmt.Sprintf("%.8f", price)
	case price < 0.001:
		// 低价 meme coin: NEIRO, HMSTR, HOT, NOT
		// 0.00015060 → "0.000151" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 0.01:
		// 中低价币: PEPE, SHIB, MEME
		// 0.00556800 → "0.005568" (6位小数)
		return fmt.Sprintf("%.6f", price)
	case price < 1.0:
		// 低价币: ASTER, DOGE, ADA, TRX
		// 0.9954 → "0.9954" (4位小数)
		return fmt.Sprintf("%.4f", price)
	case price < 100:
		// 中价币: SOL, AVAX, LINK, MATIC
		// 23.4567 → "23.4567" (4位小数)
		return fmt.Sprintf("%.4f", price)
	default:
		// 高价币: BTC, ETH (节省 Token)
		// 45678.9123 → "45678.91" (2位小数)
		return fmt.Sprintf("%.2f", price)
	}
}

// checkIchimokuAlerts 检查Ichimoku相关的交易信号并生成警报
func checkIchimokuAlerts(symbol string, current, previous *IchimokuData, alertsChan chan<- Alert) {
	if current == nil || previous == nil {
		return
	}

	// 1. 检查Tenkan-Kijun交叉信号 (金叉/死叉)
	if config.AlertThresholds.IchimokuGoldenCross {
		// 金叉信号: Tenkan从下方穿越Kijun
		if current.TenkanSen > current.KijunSen && previous.TenkanSen <= previous.KijunSen {
			alert := Alert{
				Type:      "ichimoku_golden_cross",
				Symbol:    symbol,
				Value:     current.TenkanSen,
				Threshold: current.KijunSen,
				Message:   fmt.Sprintf("Ichimoku Golden Cross: Tenkan-Sen (%.4f) crosses above Kijun-Sen (%.4f)", current.TenkanSen, current.KijunSen),
				Timestamp: time.Now(),
			}
			select {
			case alertsChan <- alert:
			default:
				// 通道满了，跳过这个警报
			}
		}
	}

	if config.AlertThresholds.IchimokuDeadCross {
		// 死叉信号: Tenkan从上方穿越Kijun
		if current.TenkanSen < current.KijunSen && previous.TenkanSen >= previous.KijunSen {
			alert := Alert{
				Type:      "ichimoku_dead_cross",
				Symbol:    symbol,
				Value:     current.TenkanSen,
				Threshold: current.KijunSen,
				Message:   fmt.Sprintf("Ichimoku Dead Cross: Tenkan-Sen (%.4f) crosses below Kijun-Sen (%.4f)", current.TenkanSen, current.KijunSen),
				Timestamp: time.Now(),
			}
			select {
			case alertsChan <- alert:
			default:
			}
		}
	}

	// 2. 检查云层突破信号
	if config.AlertThresholds.IchimokuCloudBreak {
		if current.PricePosition != previous.PricePosition {
			var message string
			var alertType string
			
			switch {
			case current.PricePosition == "above" && previous.PricePosition != "above":
				alertType = "ichimoku_cloud_break_up"
				message = fmt.Sprintf("Price breaks above Ichimoku Cloud (from %s to above)", previous.PricePosition)
			case current.PricePosition == "below" && previous.PricePosition != "below":
				alertType = "ichimoku_cloud_break_down"
				message = fmt.Sprintf("Price breaks below Ichimoku Cloud (from %s to below)", previous.PricePosition)
			case current.PricePosition == "inside":
				alertType = "ichimoku_cloud_entry"
				message = fmt.Sprintf("Price enters Ichimoku Cloud (from %s to inside)", previous.PricePosition)
			default:
				return // 不生成警报
			}
			
			alert := Alert{
				Type:      alertType,
				Symbol:    symbol,
				Value:     current.CloudThickness,
				Threshold: config.AlertThresholds.CloudThicknessMin,
				Message:   message,
				Timestamp: time.Now(),
			}
			select {
			case alertsChan <- alert:
			default:
			}
		}
	}

	// 3. 检查云层扭转信号 (云颜色变化)
	if current.CloudColor != previous.CloudColor && current.CloudColor != "neutral" {
		alertType := "ichimoku_cloud_twist"
		message := fmt.Sprintf("Ichimoku Cloud twist: Color changed from %s to %s", previous.CloudColor, current.CloudColor)
		
		alert := Alert{
			Type:      alertType,
			Symbol:    symbol,
			Value:     current.CloudThickness,
			Threshold: config.AlertThresholds.CloudThicknessMin,
			Message:   message,
			Timestamp: time.Now(),
		}
		select {
		case alertsChan <- alert:
		default:
		}
	}

	// 4. 检查云层厚度变化 (薄云更容易突破)
	if current.CloudThickness < config.AlertThresholds.CloudThicknessMin &&
	   previous.CloudThickness >= config.AlertThresholds.CloudThicknessMin {
		alert := Alert{
			Type:      "ichimoku_thin_cloud",
			Symbol:    symbol,
			Value:     current.CloudThickness,
			Threshold: config.AlertThresholds.CloudThicknessMin,
			Message:   fmt.Sprintf("Ichimoku Cloud becomes thin (%.6f), potential breakout zone", current.CloudThickness),
			Timestamp: time.Now(),
		}
		select {
		case alertsChan <- alert:
		default:
		}
	}

	// 5. 检查趋势方向变化
	if current.TrendDirection != previous.TrendDirection && current.TrendDirection != "sideways" {
		alert := Alert{
			Type:      "ichimoku_trend_change",
			Symbol:    symbol,
			Value:     0, // 趋势变化没有数值
			Threshold: 0,
			Message:   fmt.Sprintf("Ichimoku trend change: %s to %s", previous.TrendDirection, current.TrendDirection),
			Timestamp: time.Now(),
		}
		select {
		case alertsChan <- alert:
		default:
		}
	}
}

// calculateIchimoku 计算Ichimoku Cloud指标
func calculateIchimoku(klines []Kline) *IchimokuData {
	if len(klines) < 52 { // 至少需要52个数据点来计算所有组件
		return &IchimokuData{
			CloudColor:     "neutral",
			PricePosition:  "unknown",
			TrendDirection: "sideways",
		}
	}

	// 计算当前值
	tenkanSen := calculateTenkanSen(klines, 9)
	kijunSen := calculateKijunSen(klines, 26)
	senkouSpanA := calculateSenkouSpanA(tenkanSen, kijunSen)
	senkouSpanB := calculateSenkouSpanB(klines, 52)
	chikouSpan := calculateChikouSpan(klines, 26)

	// 计算云的属性
	cloudColor := "neutral"
	if senkouSpanA > senkouSpanB {
		cloudColor = "bullish"
	} else if senkouSpanA < senkouSpanB {
		cloudColor = "bearish"
	}

	cloudThickness := math.Abs(senkouSpanA - senkouSpanB)
	
	// 确定价格相对云的位置
	currentPrice := klines[len(klines)-1].Close
	pricePosition := determinePricePosition(currentPrice, senkouSpanA, senkouSpanB)
	
	// 确定趋势方向
	trendDirection := determineTrendDirection(tenkanSen, kijunSen, senkouSpanA, senkouSpanB, currentPrice)

	// 计算历史序列 (最近10个数据点)
	tenkanSeries := calculateIchimokuSeries(klines, 9, calculateTenkanSenForPeriod)
	kijunSeries := calculateIchimokuSeries(klines, 26, calculateKijunSenForPeriod)
	spanASeries := calculateSenkouSpanASeries(klines)
	spanBSeries := calculateIchimokuSeries(klines, 52, calculateSenkouSpanBForPeriod)

	return &IchimokuData{
		TenkanSen:         tenkanSen,
		KijunSen:          kijunSen,
		SenkouSpanA:       senkouSpanA,
		SenkouSpanB:       senkouSpanB,
		ChikouSpan:        chikouSpan,
		CloudColor:        cloudColor,
		CloudThickness:    cloudThickness,
		PricePosition:     pricePosition,
		TrendDirection:    trendDirection,
		TenkanSenSeries:   tenkanSeries,
		KijunSenSeries:    kijunSeries,
		SenkouSpanASeries: spanASeries,
		SenkouSpanBSeries: spanBSeries,
	}
}

// calculateTenkanSen 计算转换线 (Tenkan-Sen) - 9期
func calculateTenkanSen(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	
	start := len(klines) - period
	highest := klines[start].High
	lowest := klines[start].Low
	
	for i := start + 1; i < len(klines); i++ {
		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}
	
	return (highest + lowest) / 2
}

// calculateKijunSen 计算基准线 (Kijun-Sen) - 26期
func calculateKijunSen(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	
	start := len(klines) - period
	highest := klines[start].High
	lowest := klines[start].Low
	
	for i := start + 1; i < len(klines); i++ {
		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}
	
	return (highest + lowest) / 2
}

// calculateSenkouSpanA 计算先行带A (Senkou Span A)
func calculateSenkouSpanA(tenkanSen, kijunSen float64) float64 {
	return (tenkanSen + kijunSen) / 2
}

// calculateSenkouSpanB 计算先行带B (Senkou Span B) - 52期
func calculateSenkouSpanB(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	
	start := len(klines) - period
	highest := klines[start].High
	lowest := klines[start].Low
	
	for i := start + 1; i < len(klines); i++ {
		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}
	
	return (highest + lowest) / 2
}

// calculateChikouSpan 计算迟行线 (Chikou Span)
func calculateChikouSpan(klines []Kline, displacement int) float64 {
	if len(klines) < displacement {
		return 0
	}
	
	// 迟行线是当前价格向前位移26期
	return klines[len(klines)-1].Close
}

// determinePricePosition 确定价格相对云的位置
func determinePricePosition(price, spanA, spanB float64) string {
	cloudTop := math.Max(spanA, spanB)
	cloudBottom := math.Min(spanA, spanB)
	
	if price > cloudTop {
		return "above"
	} else if price < cloudBottom {
		return "below"
	} else {
		return "inside"
	}
}

// determineTrendDirection 确定趋势方向
func determineTrendDirection(tenkan, kijun, spanA, spanB, price float64) string {
	// 多重条件判断趋势
	bullishSignals := 0
	bearishSignals := 0
	
	// 1. Tenkan vs Kijun
	if tenkan > kijun {
		bullishSignals++
	} else if tenkan < kijun {
		bearishSignals++
	}
	
	// 2. 云的颜色
	if spanA > spanB {
		bullishSignals++
	} else if spanA < spanB {
		bearishSignals++
	}
	
	// 3. 价格相对云的位置
	cloudTop := math.Max(spanA, spanB)
	cloudBottom := math.Min(spanA, spanB)
	if price > cloudTop {
		bullishSignals++
	} else if price < cloudBottom {
		bearishSignals++
	}
	
	if bullishSignals > bearishSignals {
		return "bullish"
	} else if bearishSignals > bullishSignals {
		return "bearish"
	} else {
		return "sideways"
	}
}

// calculateIchimokuSeries 计算Ichimoku组件的历史序列
func calculateIchimokuSeries(klines []Kline, period int, calcFunc func([]Kline, int, int) float64) []float64 {
	series := make([]float64, 0, 10)
	
	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(klines); i++ {
		if i >= period-1 {
			value := calcFunc(klines, period, i)
			series = append(series, value)
		}
	}
	
	return series
}

// calculateTenkanSenForPeriod 为特定索引计算Tenkan-Sen
func calculateTenkanSenForPeriod(klines []Kline, period, endIndex int) float64 {
	if endIndex < period-1 {
		return 0
	}
	
	start := endIndex - period + 1
	highest := klines[start].High
	lowest := klines[start].Low
	
	for i := start + 1; i <= endIndex; i++ {
		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}
	
	return (highest + lowest) / 2
}

// calculateKijunSenForPeriod 为特定索引计算Kijun-Sen
func calculateKijunSenForPeriod(klines []Kline, period, endIndex int) float64 {
	if endIndex < period-1 {
		return 0
	}
	
	start := endIndex - period + 1
	highest := klines[start].High
	lowest := klines[start].Low
	
	for i := start + 1; i <= endIndex; i++ {
		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}
	
	return (highest + lowest) / 2
}

// calculateSenkouSpanBForPeriod 为特定索引计算Senkou Span B
func calculateSenkouSpanBForPeriod(klines []Kline, period, endIndex int) float64 {
	if endIndex < period-1 {
		return 0
	}
	
	start := endIndex - period + 1
	highest := klines[start].High
	lowest := klines[start].Low
	
	for i := start + 1; i <= endIndex; i++ {
		if klines[i].High > highest {
			highest = klines[i].High
		}
		if klines[i].Low < lowest {
			lowest = klines[i].Low
		}
	}
	
	return (highest + lowest) / 2
}

// calculateSenkouSpanASeries 计算Senkou Span A的历史序列
func calculateSenkouSpanASeries(klines []Kline) []float64 {
	series := make([]float64, 0, 10)
	
	// 获取最近10个数据点
	start := len(klines) - 10
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(klines); i++ {
		if i >= 25 { // 需要至少26个数据点来计算Kijun-Sen
			tenkan := calculateTenkanSenForPeriod(klines, 9, i)
			kijun := calculateKijunSenForPeriod(klines, 26, i)
			spanA := (tenkan + kijun) / 2
			series = append(series, spanA)
		}
	}
	
	return series
}

// generateIchimokuSignals 生成Ichimoku交易信号
func generateIchimokuSignals(current, previous *IchimokuData) []IchimokuSignal {
	var signals []IchimokuSignal
	
	if previous == nil {
		return signals
	}
	
	// 1. Tenkan-Kijun交叉信号
	if current.TenkanSen > current.KijunSen && previous.TenkanSen <= previous.KijunSen {
		signals = append(signals, IchimokuSignal{
			Type:        "golden_cross",
			Strength:    75.0,
			Description: "Tenkan-Sen crosses above Kijun-Sen (Golden Cross)",
			Timestamp:   time.Now(),
		})
	} else if current.TenkanSen < current.KijunSen && previous.TenkanSen >= previous.KijunSen {
		signals = append(signals, IchimokuSignal{
			Type:        "dead_cross",
			Strength:    75.0,
			Description: "Tenkan-Sen crosses below Kijun-Sen (Dead Cross)",
			Timestamp:   time.Now(),
		})
	}
	
	// 2. 云层突破信号
	if current.PricePosition != previous.PricePosition {
		strength := 60.0
		if current.CloudThickness > previous.CloudThickness {
			strength = 80.0 // 厚云突破更强
		}
		
		signals = append(signals, IchimokuSignal{
			Type:        "cloud_break",
			Strength:    strength,
			Description: fmt.Sprintf("Price moved from %s to %s cloud", previous.PricePosition, current.PricePosition),
			Timestamp:   time.Now(),
		})
	}
	
	// 3. 云颜色变化信号
	if current.CloudColor != previous.CloudColor && current.CloudColor != "neutral" {
		signals = append(signals, IchimokuSignal{
			Type:        "cloud_twist",
			Strength:    65.0,
			Description: fmt.Sprintf("Cloud color changed to %s", current.CloudColor),
			Timestamp:   time.Now(),
		})
	}
	
	return signals
}

// formatFloatSlice 格式化float64切片为字符串（使用动态精度）
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = formatPriceWithDynamicPrecision(v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}
