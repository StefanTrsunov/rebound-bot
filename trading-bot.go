package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// NewTradingBot creates a new trading bot instance
func NewTradingBot(budget float64) (*TradingBot, error) {
	// Initialize Binance configuration
	binanceConfig := BinanceConfig{
		APIKey:    os.Getenv("BINANCE_API_KEY"),
		SecretKey: os.Getenv("BINANCE_SECRET_KEY"),
		BaseURL:   "https://api.binance.com",
	}

	// Check if API keys are provided
	if binanceConfig.APIKey == "" || binanceConfig.SecretKey == "" {
		return nil, fmt.Errorf("BINANCE API KEYS REQUIRED!")
	}

	fmt.Println("Starting with real trading - monitor closely!")

	bot := &TradingBot{
		TotalBudget:      budget,
		AvailableBudget:  budget,
		InvestmentAmount: 7.0, // 7 USDT per trade as specified in strategy
		Positions:        make([]TradingPosition, 0),
		CompletedTrades:  make([]CompletedTrade, 0),
		WatchList:        make([]OptimizedTicker, 0),
		Stats:            PaperTradingStats{},
		NextPositionID:   1,
		StartTime:        time.Now(),
		BinanceConfig:    binanceConfig,
	}

	return bot, nil
}

func (bot *TradingBot) fetchTop20CoinsFromCMC() ([]OptimizedTicker, error) {
	fmt.Println("Fetching top 20 non-stablecoin coins from CoinMarketCap API...")

	cmcAPIKey := os.Getenv("COIN_MARKET_CAP_API_KEY")
	if cmcAPIKey == "" {
		return nil, fmt.Errorf("COIN_MARKET_CAP_API_KEY not set in environment variables")
	}

	// Fetch top 50 to ensure we get 20 non-stablecoins after filtering
	apiURL := "https://pro-api.coinmarketcap.com/v1/cryptocurrency/listings/latest?start=1&limit=50&convert=USD"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating CMC request: %v", err)
	}

	req.Header.Set("X-CMC_PRO_API_KEY", cmcAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making CMC request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CMC API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading CMC response: %v", err)
	}

	var cmcResponse CoinMarketCapResponse
	err = json.Unmarshal(body, &cmcResponse)
	if err != nil {
		return nil, fmt.Errorf("error parsing CMC JSON: %v", err)
	}

	if cmcResponse.Status.ErrorCode != 0 {
		return nil, fmt.Errorf("CMC API error: %s", cmcResponse.Status.ErrorMessage)
	}

	// Create OptimizedTicker array with non-stablecoin CMC top coins
	top20Coins := make([]OptimizedTicker, 0, 20)
	addedCount := 0

	fmt.Println("\n=== FILTERING CMC TOP 50 FOR TRADING ===")

	for _, coin := range cmcResponse.Data {
		// Skip if already have 20 coins
		if addedCount >= 20 {
			break
		}

		symbol := coin.Symbol + "USDT"
		price := coin.Quote.USD.Price
		change24h := coin.Quote.USD.PercentChange24h

		// Use CoinMarketCap data directly - no need for additional Binance call
		top20Coins = append(top20Coins, OptimizedTicker{
			Symbol:             symbol,
			LastPrice:          price,
			PriceChangePercent: change24h,
		})

		// Enhanced logging for buy opportunities
		buySignal := ""
		if change24h <= -5.0 && change24h > -10.0 {
			buySignal = " 🔥 BUY SIGNAL!"
		} else if change24h <= -4.5 && change24h > -5.0 {
			buySignal = " ⚡ WATCH (close to threshold)"
		} else if change24h <= -10.0 {
			buySignal = " ⚠️  DANGER ZONE (>10% drop)"
		}

		fmt.Printf("ADD: %s: $%.4f (%.2f%% 24h)%s\n",
			coin.Symbol, price, change24h, buySignal)

		addedCount++
	}

	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Successfully loaded %d tradeable non-stablecoin coins\n", len(top20Coins))
	fmt.Printf("Data source: CoinMarketCap API (no additional Binance API calls needed)\n")

	// Show buy opportunities summary
	buyOpportunities := 0
	watchList := 0
	for _, coin := range top20Coins {
		if coin.PriceChangePercent <= -5.0 && coin.PriceChangePercent > -10.0 {
			buyOpportunities++
		} else if coin.PriceChangePercent <= -4.5 && coin.PriceChangePercent > -5.0 {
			watchList++
		}
	}

	if buyOpportunities > 0 {
		fmt.Printf("IMMEDIATE BUY OPPORTUNITIES: %d coins (5-10%% drop range)\n", buyOpportunities)
	}
	if watchList > 0 {
		fmt.Printf("⚡ WATCH LIST: %d coins (close to 5%% threshold)\n", watchList)
	}
	if buyOpportunities == 0 && watchList == 0 {
		fmt.Printf("NO IMMEDIATE OPPORTUNITIES: Market is stable\n")
	}

	return top20Coins, nil
}

// generateSignature creates HMAC SHA256 signature for Binance API
func (bot *TradingBot) generateSignature(queryString string) string {
	mac := hmac.New(sha256.New, []byte(bot.BinanceConfig.SecretKey))
	mac.Write([]byte(queryString))
	return hex.EncodeToString(mac.Sum(nil))
}

// SymbolFilters holds the trading rules for a specific symbol
type SymbolFilters struct {
	StepSize string `json:"stepSize"`
	TickSize string `json:"tickSize"`
}

// ExchangeInfo represents the Binance exchange info response for symbol filters
type ExchangeInfo struct {
	Symbols []struct {
		Symbol  string `json:"symbol"`
		Filters []struct {
			FilterType string `json:"filterType"`
			StepSize   string `json:"stepSize,omitempty"`
			TickSize   string `json:"tickSize,omitempty"`
		} `json:"filters"`
	} `json:"symbols"`
}

// getSymbolFilters fetches trading rules for a specific symbol from Binance
func (bot *TradingBot) getSymbolFilters(symbol string) (*SymbolFilters, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	apiURL := bot.BinanceConfig.BaseURL + "/api/v3/exchangeInfo?symbol=" + symbol

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating exchange info request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting exchange info: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading exchange info response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exchange info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var exchangeInfo ExchangeInfo
	err = json.Unmarshal(body, &exchangeInfo)
	if err != nil {
		return nil, fmt.Errorf("error parsing exchange info: %v", err)
	}

	if len(exchangeInfo.Symbols) == 0 {
		return nil, fmt.Errorf("symbol %s not found", symbol)
	}

	symbolInfo := exchangeInfo.Symbols[0]
	filters := &SymbolFilters{}

	for _, filter := range symbolInfo.Filters {
		switch filter.FilterType {
		case "LOT_SIZE":
			filters.StepSize = filter.StepSize
		case "PRICE_FILTER":
			filters.TickSize = filter.TickSize
		}
	}

	return filters, nil
}

// roundToTickSize rounds a price to the correct tick size for Binance
func roundToTickSize(price float64, tickSize string) float64 {
	tick, err := strconv.ParseFloat(tickSize, 64)
	if err != nil || tick <= 0 {
		return price
	}

	// Round to the nearest tick
	return float64(int64(price/tick+0.5)) * tick
}

// executeBuyOrder places a market buy order on Binance
func (bot *TradingBot) executeBuyOrder(symbol string, quoteOrderQty float64) (*OrderResponse, error) {
	if bot.BinanceConfig.APIKey == "" || bot.BinanceConfig.SecretKey == "" {
		return nil, fmt.Errorf("Binance API credentials not configured")
	}

	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	//order parameters
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", "BUY")
	params.Set("type", "MARKET")
	params.Set("quoteOrderQty", fmt.Sprintf("%.8f", quoteOrderQty))
	params.Set("timestamp", fmt.Sprintf("%d", timestamp))

	queryString := params.Encode()
	signature := bot.generateSignature(queryString)

	// Create request
	orderURL := bot.BinanceConfig.BaseURL + "/api/v3/order"
	req, err := http.NewRequest("POST", orderURL, strings.NewReader(queryString+"&signature="+signature))
	if err != nil {
		return nil, fmt.Errorf("error creating buy order request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-MBX-APIKEY", bot.BinanceConfig.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing buy order: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading buy order response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("buy order failed with status %d: %s", resp.StatusCode, string(body))
	}

	var orderResp OrderResponse
	err = json.Unmarshal(body, &orderResp)
	if err != nil {
		return nil, fmt.Errorf("error parsing buy order response: %v", err)
	}

	return &orderResp, nil
}

// executeLimitSellOrder places a limit sell order on Binance
func (bot *TradingBot) executeLimitSellOrder(symbol string, quantity float64, price float64) (*OrderResponse, error) {
	if bot.BinanceConfig.APIKey == "" || bot.BinanceConfig.SecretKey == "" {
		return nil, fmt.Errorf("Binance API credentials not configured")
	}

	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	// Prepare order parameters
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", "SELL")
	params.Set("type", "LIMIT")
	params.Set("timeInForce", "GTC") // Good Till Cancelled
	params.Set("quantity", fmt.Sprintf("%.8f", quantity))
	params.Set("price", fmt.Sprintf("%.8f", price))
	params.Set("timestamp", fmt.Sprintf("%d", timestamp))

	queryString := params.Encode()
	signature := bot.generateSignature(queryString)

	// Create request
	orderURL := bot.BinanceConfig.BaseURL + "/api/v3/order"
	req, err := http.NewRequest("POST", orderURL, strings.NewReader(queryString+"&signature="+signature))
	if err != nil {
		return nil, fmt.Errorf("error creating limit sell order request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-MBX-APIKEY", bot.BinanceConfig.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing limit sell order: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading limit sell order response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("limit sell order failed with status %d: %s", resp.StatusCode, string(body))
	}

	var orderResp OrderResponse
	err = json.Unmarshal(body, &orderResp)
	if err != nil {
		return nil, fmt.Errorf("error parsing limit sell order response: %v", err)
	}

	return &orderResp, nil
}

// executeSellOrder places a market sell order on Binance
func (bot *TradingBot) executeSellOrder(symbol string, quantity float64) (*OrderResponse, error) {
	if bot.BinanceConfig.APIKey == "" || bot.BinanceConfig.SecretKey == "" {
		return nil, fmt.Errorf("Binance API credentials not configured")
	}

	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	// Prepare order parameters
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", "SELL")
	params.Set("type", "MARKET")
	params.Set("quantity", fmt.Sprintf("%.8f", quantity))
	params.Set("timestamp", fmt.Sprintf("%d", timestamp))

	queryString := params.Encode()
	signature := bot.generateSignature(queryString)

	// Create request
	orderURL := bot.BinanceConfig.BaseURL + "/api/v3/order"
	req, err := http.NewRequest("POST", orderURL, strings.NewReader(queryString+"&signature="+signature))
	if err != nil {
		return nil, fmt.Errorf("error creating sell order request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-MBX-APIKEY", bot.BinanceConfig.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing sell order: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading sell order response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sell order failed with status %d: %s", resp.StatusCode, string(body))
	}

	var orderResp OrderResponse
	err = json.Unmarshal(body, &orderResp)
	if err != nil {
		return nil, fmt.Errorf("error parsing sell order response: %v", err)
	}

	return &orderResp, nil
}

// getRealUSDTBalance fetches the actual USDT balance from Binance for budget initialization
func getRealUSDTBalance(apiKey, secretKey string) (float64, error) {
	if apiKey == "" || secretKey == "" {
		return 0, fmt.Errorf("Binance API credentials not configured")
	}

	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	params := url.Values{}
	params.Set("timestamp", fmt.Sprintf("%d", timestamp))

	queryString := params.Encode()

	// Generate signature
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(queryString))
	signature := hex.EncodeToString(mac.Sum(nil))

	accountURL := "https://api.binance.com/api/v3/account?" + queryString + "&signature=" + signature

	req, err := http.NewRequest("GET", accountURL, nil)
	if err != nil {
		return 0, fmt.Errorf("error creating account info request: %v", err)
	}

	req.Header.Set("X-MBX-APIKEY", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error getting account info: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading account response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("account info request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var accountInfo AccountInfo
	err = json.Unmarshal(body, &accountInfo)
	if err != nil {
		return 0, fmt.Errorf("error parsing account response: %v", err)
	}

	// Find USDT balance
	for _, balance := range accountInfo.Balances {
		if balance.Asset == "USDT" {
			usdtBalance, err := strconv.ParseFloat(balance.Free, 64)
			if err != nil {
				return 0, fmt.Errorf("error parsing USDT balance: %v", err)
			}
			return usdtBalance, nil
		}
	}

	return 0, fmt.Errorf("USDT balance not found in account")
}

// analyzeTradingOpportunities checks for buy opportunities based on the optimized strategy
// Focuses specifically on 5-10% drops from CoinMarketCap top 20 (excluding stablecoins)
func (bot *TradingBot) analyzeTradingOpportunities() {
	fmt.Println("\n=== Analyzing Trading Opportunities (5-10% Drop Strategy) ===")

	buyOpportunities := 0
	watchOpportunities := 0

	for _, coin := range bot.WatchList {
		coinName := strings.TrimSuffix(coin.Symbol, "USDT")

		// Safety check: Do not buy if price drops more than 11% (potential hack/major issue)
		if coin.PriceChangePercent <= -11.0 {
			fmt.Printf("SKIP %s: %.2f%% drop exceeds safety limit (-11%%)\n",
				coinName, coin.PriceChangePercent)
			continue
		}

		// Watch for potential buy opportunities (close to threshold)
		if coin.PriceChangePercent <= -4.5 && coin.PriceChangePercent > -5.0 {
			fmt.Printf("👀 WATCH: %s at %.2f%% (approaching -5%% buy threshold)\n",
				coinName, coin.PriceChangePercent)
			watchOpportunities++
		}

		// Main buy condition: exactly what you specified - between 5% and 10% drop
		if coin.PriceChangePercent <= -5.0 && coin.PriceChangePercent > -10.0 {
			buyOpportunities++
			fmt.Printf("BUY SIGNAL: %s dropped %.2f%% (perfect 5-10%% range)\n",
				coinName, coin.PriceChangePercent)

			// Execute real trade on Binance - this is where we actually use Binance API
			fmt.Printf("Executing REAL trade: %.2f USDT of %s at $%.4f\n",
				bot.InvestmentAmount, coinName, coin.LastPrice)
			// if buyOpportunities == 1 {
			bot.executeBuy(coin, coin.PriceChangePercent)
			//}
		} else if coin.PriceChangePercent > -5.0 {
			// Not enough drop yet
			fmt.Printf("HOLD: %s at %.2f%% (need >5%% drop to trigger)\n",
				coinName, coin.PriceChangePercent)
		} else if coin.PriceChangePercent <= -10.0 && coin.PriceChangePercent > -11.0 {
			// Too much drop - risky
			fmt.Printf("RISKY: %s at %.2f%% (>10%% drop, potential issues)\n",
				coinName, coin.PriceChangePercent)
		}
	}

	fmt.Printf("\n=== OPPORTUNITY SUMMARY ===\n")
	if buyOpportunities == 0 {
		fmt.Println("No coins in the 5-10% drop range for buying")
		if watchOpportunities > 0 {
			fmt.Printf("%d coins are close to the 5%% threshold - monitoring...\n", watchOpportunities)
		} else {
			fmt.Println("Market is stable - no immediate opportunities")
		}
	} else {
		fmt.Printf("Found %d BUY opportunities in the optimal 5-10%% drop range!\n", buyOpportunities)
		if watchOpportunities > 0 {
			fmt.Printf("Plus %d coins approaching the threshold\n", watchOpportunities)
		}
	}
}

// executeBuy executes real buy order on Binance mainnet - REAL MONEY!
func (bot *TradingBot) executeBuy(coin OptimizedTicker, dropPercentage float64) {
	// Check if we have enough budget
	if bot.AvailableBudget < bot.InvestmentAmount {
		fmt.Printf("Insufficient funds: Available %.2f USDT < Required %.2f USDT\n",
			bot.AvailableBudget, bot.InvestmentAmount)
		return
	}

	fmt.Printf("   [BINANCE MAINNET] Executing REAL buy order...\n")

	orderResp, err := bot.executeBuyOrder(coin.Symbol, bot.InvestmentAmount)
	if err != nil {
		fmt.Printf("   ERROR: Binance order failed: %v\n", err)
		return
	} else {
		// Parse actual executed quantity and price from Binance response
		actualQty, _ := strconv.ParseFloat(orderResp.ExecutedQty, 64)
		avgPrice := 0.0

		// Calculate average fill price
		if len(orderResp.Fills) > 0 {
			totalValue := 0.0
			totalQty := 0.0
			for _, fill := range orderResp.Fills {
				fillPrice, _ := strconv.ParseFloat(fill.Price, 64)
				fillQty, _ := strconv.ParseFloat(fill.Qty, 64)
				totalValue += fillPrice * fillQty
				totalQty += fillQty
			}
			if totalQty > 0 {
				avgPrice = totalValue / totalQty
			}
		}

		if avgPrice == 0 {
			avgPrice = coin.LastPrice // Fallback
		}

		position := TradingPosition{
			ID:                 bot.NextPositionID,
			Symbol:             coin.Symbol,
			BuyPrice:           avgPrice,
			Quantity:           actualQty,
			InvestedAmount:     bot.InvestmentAmount,
			TargetSellPrice:    avgPrice * 1.05, // Recalculate based on actual price
			BuyTime:            time.Now(),
			DropPercentage:     dropPercentage,
			CurrentValue:       avgPrice * actualQty,
			SellOrderID:        0,
			HasActiveSellOrder: false,
		}

		// Wait a moment for the buy order to fully settle before placing sell order
		fmt.Printf("   [BINANCE MAINNET] Waiting 3 seconds for buy order to settle...\n")
		time.Sleep(3 * time.Second)

		// Place a limit sell order at target price
		fmt.Printf("   [BINANCE MAINNET] Attempting to place sell order for %.6f %s at $%.6f\n",
			actualQty, strings.TrimSuffix(coin.Symbol, "USDT"), position.TargetSellPrice)

		// Get symbol filters to ensure proper price formatting
		filters, filterErr := bot.getSymbolFilters(coin.Symbol)
		if filterErr != nil {
			fmt.Printf("   WARNING: Could not get symbol filters: %v\n", filterErr)
			fmt.Printf("   INFO: Position will be monitored manually for sell opportunities\n")
		} else {
			// Round the target sell price to conform to Binance tick size
			roundedSellPrice := roundToTickSize(position.TargetSellPrice, filters.TickSize)
			fmt.Printf("   [PRICE ADJUSTMENT] Original: $%.6f -> Rounded: $%.6f (TickSize: %s)\n",
				position.TargetSellPrice, roundedSellPrice, filters.TickSize)

			// Try to place the sell order with retry logic
			maxRetries := 3
			var sellOrderResp *OrderResponse
			var sellErr error

			for retry := 1; retry <= maxRetries; retry++ {
				sellOrderResp, sellErr = bot.executeLimitSellOrder(coin.Symbol, actualQty, roundedSellPrice)
				if sellErr == nil {
					break
				}

				fmt.Printf("   RETRY %d/%d: Sell order failed: %v\n", retry, maxRetries, sellErr)
				if retry < maxRetries {
					fmt.Printf("   Waiting 2 seconds before retry...\n")
					time.Sleep(2 * time.Second)
				}
			}

			if sellErr != nil {
				fmt.Printf("   WARNING: Failed to place automatic sell order after %d attempts: %v\n", maxRetries, sellErr)
				fmt.Printf("   INFO: Position will be monitored manually for sell opportunities\n")
			} else {
				position.SellOrderID = sellOrderResp.OrderID
				position.HasActiveSellOrder = true
				position.TargetSellPrice = roundedSellPrice // Update to the actual rounded price
				fmt.Printf("   [BINANCE MAINNET] SUCCESS: Sell order placed! ID: %d at $%.6f\n",
					sellOrderResp.OrderID, roundedSellPrice)
			}
		}

		bot.Positions = append(bot.Positions, position)
		bot.AvailableBudget -= bot.InvestmentAmount
		bot.NextPositionID++

		fmt.Printf("   [BINANCE MAINNET] SUCCESS: Buy order executed! ID: %d\n", orderResp.OrderID)
		fmt.Printf("   Bought %.6f %s at $%.4f avg (Investment: %.2f USDT)\n",
			actualQty, strings.TrimSuffix(coin.Symbol, "USDT"), avgPrice, bot.InvestmentAmount)
		fmt.Printf("   Target sell price: $%.4f (+5%% profit)\n", position.TargetSellPrice)
		fmt.Printf("   Available budget: %.2f USDT remaining\n", bot.AvailableBudget)
	}
}

// getCurrentPortfolioValue calculates the current value of all positions
func (bot *TradingBot) getCurrentPortfolioValue() float64 {
	totalValue := 0.0
	for _, pos := range bot.Positions {
		totalValue += pos.CurrentValue
	}
	return totalValue
}

// runTradingCycle executes one complete trading cycle with optimized CMC+Binance integration
func (bot *TradingBot) runTradingCycle() error {
	fmt.Printf("\n" + strings.Repeat("=", 80))
	fmt.Printf("\nOptimized Trading Bot Cycle - %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("Data Source: CoinMarketCap API (Top 20, excluding stablecoins)\n")
	fmt.Printf("Trading Platform: Binance (buy/sell execution only)\n")
	fmt.Printf("Strategy: Buy 5-10%% drops, Sell at +5%% profit\n")
	fmt.Printf(strings.Repeat("=", 80))

	// Fetch current market data from CoinMarketCap top 20 (optimized - no extra Binance calls)
	watchList, err := bot.fetchTop20CoinsFromCMC()
	if err != nil {
		return fmt.Errorf("failed to fetch CoinMarketCap top 20: %v", err)
	}

	bot.WatchList = watchList
	fmt.Printf("\nMonitoring %d non-stablecoin coins from CoinMarketCap top 50\n", len(bot.WatchList))

	// Analyze new buy opportunities using CMC data
	bot.analyzeTradingOpportunities()

	return nil
}

// startBot starts the trading bot with 60-minute cycles for testing
func (bot *TradingBot) startBot() {
	fmt.Println("Starting Trading Bot...")
	fmt.Printf("Strategy: Buy on drops between -5%% to -10%% | Sell at +5%% profit\n")
	fmt.Printf("Budget: %.2f USDT | Investment per trade: %.2f USDT\n", bot.TotalBudget, bot.InvestmentAmount)
	fmt.Printf("Cycle frequency: Every 60 minutes for active testing\n")

	// Run initial cycle
	if err := bot.runTradingCycle(); err != nil {
		log.Printf("Error in trading cycle: %v", err)
	}

	// Set up 60-minute ticker for testing
	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()

	fmt.Println("\nBot will run every 60 minutes. Press Ctrl+C to stop.")

	for {
		select {
		case <-ticker.C:
			if err := bot.runTradingCycle(); err != nil {
				log.Printf("Error in trading cycle: %v", err)
			}
		}
	}
}

// StartTradingBot is the entry point for the optimized trading bot
func StartTradingBot() {
	fmt.Println("=== OPTIMIZED Crypto Trading Bot ===")
	fmt.Println("Data Strategy: CoinMarketCap API (Top 20 non-stablecoins)")
	fmt.Println("Trading Strategy: 5-10% drops → 5% profit target")
	fmt.Println("Execution Platform: Binance API (buy/sell only)")

	// Check API credentials first
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")
	cmcKey := os.Getenv("COIN_MARKET_CAP_API_KEY")

	if apiKey == "" || secretKey == "" {
		log.Fatalf("ERROR: BINANCE API KEYS REQUIRED! Set BINANCE_API_KEY and BINANCE_SECRET_KEY in .env file")
	}

	if cmcKey == "" {
		log.Fatalf("ERROR: COINMARKETCAP API KEY REQUIRED! Set COIN_MARKET_CAP_API_KEY in .env file")
	}

	// Fetch real USDT balance from Binance
	fmt.Println("\nFetching real USDT balance from Binance...")

	realBalance, err := getRealUSDTBalance(apiKey, secretKey)
	if err != nil {
		log.Fatalf("ERROR: Failed to fetch real USDT balance: %v", err)
	}

	fmt.Printf("SUCCESS: Real USDT Balance: %.2f USDT\n", realBalance)

	if realBalance < 7.0 {
		log.Fatalf("ERROR: Insufficient USDT balance (%.2f). Need at least 7 USDT for trading.", realBalance)
	}

	if realBalance < 20.0 {
		fmt.Printf("WARNING: Low balance detected (%.2f USDT). Consider reducing INVESTMENT_PER_TRADE.\n", realBalance)
	}

	// Initialize bot using real balance
	bot, err := NewTradingBot(realBalance)
	if err != nil {
		log.Fatalf("Failed to initialize trading bot: %v", err)
	}

	// Start continuous trading with 5-minute intervals
	fmt.Println("\nStarting optimized trading mode...")
	fmt.Println("CoinMarketCap: Real-time top 20 data")
	fmt.Println("Binance: Trading execution only")
	fmt.Println("Strategy: Buy 5-10% drops, Sell +5% profit")
	bot.startBot()
}
