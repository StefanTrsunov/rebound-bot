package main

import (
	"fmt"
	"os"
	"strings"
)

func showHelp() {
	fmt.Println("Optimized Crypto Trading Bot - Available Commands")
	fmt.Println("==================================================")
	fmt.Println()
	fmt.Println("STRATEGY:")
	fmt.Println("  • Data: CoinMarketCap API (Top 20 coins, excluding stablecoins)")
	fmt.Println("  • Buy: When coins drop 5-10% in 24h")
	fmt.Println("  • Sell: When coins reach +5% profit")
	fmt.Println("  • Trading: Binance API (execution only)")
	fmt.Println()
	fmt.Println("Available Commands:")
	fmt.Println("  start             Start the automated trading bot (REAL MONEY)")
	fmt.Println("  help              Show this help message")
	fmt.Println()
	fmt.Println("Usage: ./trading-bot <command>")
	fmt.Println("Example: ./trading-bot start")
}

func main() {
	if _, err := os.Stat(".env"); err == nil {
		// Load .env file
		content, err := os.ReadFile(".env")
		if err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.Contains(line, "=") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						os.Setenv(key, value)
					}
				}
			}
		}
	}

	if len(os.Args) < 2 {
		showHelp()
		return
	}

	command := strings.ToLower(os.Args[1])

	switch command {
	case "help", "-h", "--help":
		showHelp()
	case "start":
		fmt.Println("Starting Optimized Trading Bot...")
		fmt.Println("Strategy: CoinMarketCap Top 20 (no stablecoins) + Binance execution")
		fmt.Println("Target: 5-10% drops with 5% profit targets")
		fmt.Println("Now starting the optimized trading bot...")
		StartTradingBot()
	case "test-cmc":
		fmt.Println("Testing optimized CoinMarketCap integration...")
	default:
		fmt.Printf("❌ Unknown command: %s\n", command)
		fmt.Println("Run './trading-bot help' for available commands")
	}
}
