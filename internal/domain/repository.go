package domain

type ScreenerRepository interface {
	SaveCoins(coins []CoinData)
	GetCoins() []CoinData
}
