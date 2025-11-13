package market

import (
	"context"
)

type DataProvider interface {
	GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error)

	GetFundingRate(ctx context.Context, symbol string) (float64, error)

	GetOpenInterest(ctx context.Context, symbol string) (float64, error)

	GetMarkPrice(ctx context.Context, symbol string) (float64, error)

	GetMarketData(ctx context.Context, symbol string) (*Data, error)

	Name() string
}

type ProviderMetrics struct {
	RequestsTotal   int64
	ErrorsTotal     int64
	LatencySeconds  float64
	CacheHitRatio   float64
}
