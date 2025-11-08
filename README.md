# Rebound bot

## Strategy

1. Fetch 20 coins from CMC20(CoinMarketCap 20 Index)
[cmc20](https://coinmarketcap.com/charts/cmc20/)

2. See if those coins falls between -5% and -10% in 24hr

3. If yes, buy 7 dollars worth of that coin.

4. Set automatic sell order once it buys. (+5% of the price it was bought)
