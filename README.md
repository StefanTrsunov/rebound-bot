# Rebound bot

The philosophy is that the market will always bounce back on the top coins

## Setup

You will need a `Binance KEY` `Binance SECRET KEY` and a `CoinmarketCap API KEY`

You have a `.env.example` file just put the values (keys) and you are good to go

## Strategy

1. Fetch 20 coins from CMC20(CoinMarketCap 20 Index)
[cmc20](https://coinmarketcap.com/charts/cmc20/)

2. See if those coins falls between -5% and -10% in 24hr

3. If yes, buy 7 dollars worth of that coin.

4. Set automatic sell order once it buys. (+5% of the price it was bought)
