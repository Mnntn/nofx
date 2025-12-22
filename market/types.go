package market

import "time"

// Data 市场数据结构
type Data struct {
	Symbol            string
	CurrentPrice      float64
	PriceChange1h     float64 // 1小时价格变化百分比
	PriceChange4h     float64 // 4小时价格变化百分比
	CurrentEMA20      float64
	CurrentMACD       float64
	CurrentRSI7       float64
	OpenInterest      *OIData
	FundingRate       float64
	IntradaySeries    *IntradayData
	LongerTermContext *LongerTermData
	IchimokuCloud     *IchimokuData
}

// OIData Open Interest数据
type OIData struct {
	Latest  float64
	Average float64
}

// IntradayData 日内数据(3分钟间隔)
type IntradayData struct {
	MidPrices   []float64
	EMA20Values []float64
	MACDValues  []float64
	RSI7Values  []float64
	RSI14Values   []float64
	Volume        []float64
	ATR14         float64
	IchimokuCloud *IchimokuData
}

// LongerTermData 长期数据(4小时时间框架)
type LongerTermData struct {
	EMA20         float64
	EMA50         float64
	ATR3          float64
	ATR14         float64
	CurrentVolume float64
	AverageVolume float64
	MACDValues    []float64
	RSI14Values   []float64
	IchimokuCloud *IchimokuData
}

// IchimokuData Ichimoku Cloud数据结构
type IchimokuData struct {
	TenkanSen    float64   `json:"tenkan_sen"`    // 转换线 (快线) - 9期
	KijunSen     float64   `json:"kijun_sen"`     // 基准线 (慢线) - 26期
	SenkouSpanA  float64   `json:"senkou_span_a"` // 先行带A (云的上边界/下边界)
	SenkouSpanB  float64   `json:"senkou_span_b"` // 先行带B (云的上边界/下边界)
	ChikouSpan   float64   `json:"chikou_span"`   // 迟行线
	CloudColor   string    `json:"cloud_color"`   // 云的颜色: "bullish" (绿色) 或 "bearish" (红色)
	CloudThickness float64 `json:"cloud_thickness"` // 云的厚度 (SpanA和SpanB的差值绝对值)
	PricePosition string   `json:"price_position"`  // 价格相对云的位置: "above", "below", "inside"
	TrendDirection string  `json:"trend_direction"` // 趋势方向: "bullish", "bearish", "sideways"
	
	// 历史数据序列 (用于分析趋势变化)
	TenkanSenSeries  []float64 `json:"tenkan_sen_series"`
	KijunSenSeries   []float64 `json:"kijun_sen_series"`
	SenkouSpanASeries []float64 `json:"senkou_span_a_series"`
	SenkouSpanBSeries []float64 `json:"senkou_span_b_series"`
}

// IchimokuSignal Ichimoku交易信号
type IchimokuSignal struct {
	Type        string  `json:"type"`        // 信号类型
	Strength    float64 `json:"strength"`    // 信号强度 (0-100)
	Description string  `json:"description"` // 信号描述
	Timestamp   time.Time `json:"timestamp"` // 信号时间
}

// Binance API 响应结构
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol            string `json:"symbol"`
	Status            string `json:"status"`
	BaseAsset         string `json:"baseAsset"`
	QuoteAsset        string `json:"quoteAsset"`
	ContractType      string `json:"contractType"`
	PricePrecision    int    `json:"pricePrecision"`
	QuantityPrecision int    `json:"quantityPrecision"`
}

type Kline struct {
	OpenTime            int64   `json:"openTime"`
	Open                float64 `json:"open"`
	High                float64 `json:"high"`
	Low                 float64 `json:"low"`
	Close               float64 `json:"close"`
	Volume              float64 `json:"volume"`
	CloseTime           int64   `json:"closeTime"`
	QuoteVolume         float64 `json:"quoteVolume"`
	Trades              int     `json:"trades"`
	TakerBuyBaseVolume  float64 `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume float64 `json:"takerBuyQuoteVolume"`
}

type KlineResponse []interface{}

type PriceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Ticker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
}

// 特征数据结构
type SymbolFeatures struct {
	Symbol           string    `json:"symbol"`
	Timestamp        time.Time `json:"timestamp"`
	Price            float64   `json:"price"`
	PriceChange15Min float64   `json:"price_change_15min"`
	PriceChange1H    float64   `json:"price_change_1h"`
	PriceChange4H    float64   `json:"price_change_4h"`
	Volume           float64   `json:"volume"`
	VolumeRatio5     float64   `json:"volume_ratio_5"`
	VolumeRatio20    float64   `json:"volume_ratio_20"`
	VolumeTrend      float64   `json:"volume_trend"`
	RSI14            float64   `json:"rsi_14"`
	SMA5             float64   `json:"sma_5"`
	SMA10            float64   `json:"sma_10"`
	SMA20            float64   `json:"sma_20"`
	HighLowRatio     float64   `json:"high_low_ratio"`
	Volatility20     float64   `json:"volatility_20"`
	PositionInRange  float64   `json:"position_in_range"`
}

// 警报数据结构
type Alert struct {
	Type      string    `json:"type"`
	Symbol    string    `json:"symbol"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type Config struct {
	AlertThresholds AlertThresholds `json:"alert_thresholds"`
	UpdateInterval  int             `json:"update_interval"` // seconds
	CleanupConfig   CleanupConfig   `json:"cleanup_config"`
}

type AlertThresholds struct {
	VolumeSpike      float64 `json:"volume_spike"`
	PriceChange15Min float64 `json:"price_change_15min"`
	VolumeTrend      float64 `json:"volume_trend"`
	RSIOverbought    float64 `json:"rsi_overbought"`
	RSIOversold      float64 `json:"rsi_oversold"`
	
	// Ichimoku Cloud 阈值
	IchimokuGoldenCross  bool    `json:"ichimoku_golden_cross"`  // Tenkan-Kijun金叉信号
	IchimokuDeadCross    bool    `json:"ichimoku_dead_cross"`    // Tenkan-Kijun死叉信号
	IchimokuCloudBreak   bool    `json:"ichimoku_cloud_break"`   // 价格突破云层信号
	IchimokuChikouBreak  bool    `json:"ichimoku_chikou_break"`  // Chikou突破价格信号
	CloudThicknessMin    float64 `json:"cloud_thickness_min"`    // 云层最小厚度阈值
}
type CleanupConfig struct {
	InactiveTimeout   time.Duration `json:"inactive_timeout"`    // 不活跃超时时间
	MinScoreThreshold float64       `json:"min_score_threshold"` // 最低评分阈值
	NoAlertTimeout    time.Duration `json:"no_alert_timeout"`    // 无警报超时时间
	CheckInterval     time.Duration `json:"check_interval"`      // 检查间隔
}

var config = Config{
	AlertThresholds: AlertThresholds{
		VolumeSpike:      3.0,
		PriceChange15Min: 0.05,
		VolumeTrend:      2.0,
		RSIOverbought:    70,
		RSIOversold:      30,
		
		// Ichimoku Cloud 默认阈值
		IchimokuGoldenCross:  true,  // 启用金叉信号
		IchimokuDeadCross:    true,  // 启用死叉信号
		IchimokuCloudBreak:   true,  // 启用云层突破信号
		IchimokuChikouBreak:  true,  // 启用Chikou突破信号
		CloudThicknessMin:    0.001, // 云层最小厚度 (价格的0.1%)
	},
	CleanupConfig: CleanupConfig{
		InactiveTimeout:   30 * time.Minute,
		MinScoreThreshold: 15.0,
		NoAlertTimeout:    20 * time.Minute,
		CheckInterval:     5 * time.Minute,
	},
	UpdateInterval: 60, // 1 minute
}
