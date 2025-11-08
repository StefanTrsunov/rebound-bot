package main

import "time"

// TradingPosition represents an active trading position
type TradingPosition struct {
	ID                 int // Unique position ID
	Symbol             string
	BuyPrice           float64
	Quantity           float64
	InvestedAmount     float64
	TargetSellPrice    float64
	BuyTime            time.Time
	DropPercentage     float64 // The drop percentage when bought
	CurrentValue       float64 // Current market value
	SellOrderID        int64   // Binance sell order ID (0 if no order placed)
	HasActiveSellOrder bool    // Track if sell order is active
}

// CompletedTrade represents a finished trade for performance tracking
type CompletedTrade struct {
	ID             int
	Symbol         string
	BuyPrice       float64
	SellPrice      float64
	Quantity       float64
	InvestedAmount float64
	Profit         float64
	ProfitPercent  float64
	BuyTime        time.Time
	SellTime       time.Time
	HoldDuration   time.Duration
}

// PaperTradingStats tracks performance metrics
type PaperTradingStats struct {
	TotalTrades     int
	WinningTrades   int
	LosingTrades    int
	TotalProfit     float64
	TotalLoss       float64
	NetProfit       float64
	WinRate         float64
	AverageProfit   float64
	AverageLoss     float64
	LargestWin      float64
	LargestLoss     float64
	AverageHoldTime time.Duration
}

// BinanceConfig holds API configuration for Binance
type BinanceConfig struct {
	APIKey    string
	SecretKey string
	BaseURL   string // Mainnet URL
}

// OrderResponse represents Binance order response
type OrderResponse struct {
	Symbol        string `json:"symbol"`
	OrderID       int64  `json:"orderId"`
	ClientOrderID string `json:"clientOrderId"`
	TransactTime  int64  `json:"transactTime"`
	Price         string `json:"price"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Status        string `json:"status"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	Fills         []struct {
		Price           string `json:"price"`
		Qty             string `json:"qty"`
		Commission      string `json:"commission"`
		CommissionAsset string `json:"commissionAsset"`
	} `json:"fills"`
}

// AccountInfo represents Binance account information
type AccountInfo struct {
	Balances []struct {
		Asset  string `json:"asset"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	} `json:"balances"`
}

// TradingBot represents our trading bot configuration
type TradingBot struct {
	TotalBudget      float64
	AvailableBudget  float64 // Track remaining budget
	InvestmentAmount float64 // Amount to invest per trade (5 EUR)
	Positions        []TradingPosition
	CompletedTrades  []CompletedTrade
	WatchList        []OptimizedTicker
	Stats            PaperTradingStats
	NextPositionID   int           // For unique position tracking
	StartTime        time.Time     // When trading started
	BinanceConfig    BinanceConfig // API configuration
}

// Ticker24hr represents the 24hr ticker statistics from Binance API
type Ticker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	WeightedAvgPrice   string `json:"weightedAvgPrice"`
	PrevClosePrice     string `json:"prevClosePrice"`
	LastPrice          string `json:"lastPrice"`
	LastQty            string `json:"lastQty"`
	BidPrice           string `json:"bidPrice"`
	BidQty             string `json:"bidQty"`
	AskPrice           string `json:"askPrice"`
	AskQty             string `json:"askQty"`
	OpenPrice          string `json:"openPrice"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	OpenTime           int64  `json:"openTime"`
	CloseTime          int64  `json:"closeTime"`
	Count              int64  `json:"count"`
}

// OptimizedTicker contains only the fields we actually use
type OptimizedTicker struct {
	Symbol             string
	LastPrice          float64
	PriceChangePercent float64
	PercentChange24h   float64
}

// CoinMarketCapResponse represents the response from CoinMarketCap API
type CoinMarketCapResponse struct {
	Status struct {
		Timestamp    string `json:"timestamp"`
		ErrorCode    int    `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		Elapsed      int    `json:"elapsed"`
		CreditCount  int    `json:"credit_count"`
	} `json:"status"`
	Data []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
		Slug   string `json:"slug"`
		Quote  struct {
			USD struct {
				Price            float64 `json:"price"`
				Volume24h        float64 `json:"volume_24h"`
				PercentChange1h  float64 `json:"percent_change_1h"`
				PercentChange24h float64 `json:"percent_change_24h"`
				PercentChange7d  float64 `json:"percent_change_7d"`
				MarketCap        float64 `json:"market_cap"`
				LastUpdated      string  `json:"last_updated"`
			} `json:"USD"`
		} `json:"quote"`
	} `json:"data"`
}
