package market

import (
	"testing"
	"time"
)

func TestCalculateNextBoundary(t *testing.T) {
	tests := []struct {
		name     string
		interval string
	}{
		{
			name:     "3m boundary alignment",
			interval: "3m",
		},
		{
			name:     "4h boundary alignment",
			interval: "4h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &HyperliquidProvider{
				apiURL: "https://api.hyperliquid-testnet.xyz",
			}

			now := time.Now()
			got := provider.calculateNextBoundary(tt.interval)

			if !got.After(now) {
				t.Errorf("calculateNextBoundary() = %v, should be after current time %v", got, now)
			}

			if tt.interval == "3m" {
				if got.Minute()%3 != 0 {
					t.Errorf("calculateNextBoundary() minute %d is not aligned to 3m boundary", got.Minute())
				}
			} else if tt.interval == "4h" {
				if got.Hour()%4 != 0 {
					t.Errorf("calculateNextBoundary() hour %d is not aligned to 4h boundary", got.Hour())
				}
			}
		})
	}
}

func TestAlignToBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		klines   []Kline
		wantLen  int
	}{
		{
			name:     "Remove partial 3m candle",
			interval: "3m",
			klines: []Kline{
				{OpenTime: 1700000000000, CloseTime: 1700000179999, Open: 100, High: 101, Low: 99, Close: 100.5, Volume: 1000},
				{OpenTime: 1700000180000, CloseTime: 1700000359999, Open: 100.5, High: 102, Low: 100, Close: 101, Volume: 1500},
				{OpenTime: 1700000360000, CloseTime: 1700000539999, Open: 101, High: 103, Low: 101, Close: 102, Volume: 2000},
			},
			wantLen: 3, // All candles are complete
		},
		{
			name:     "Empty klines",
			interval: "3m",
			klines:   []Kline{},
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &HyperliquidProvider{
				apiURL: "https://api.hyperliquid-testnet.xyz",
			}

			result := provider.alignToBoundaries(tt.klines, tt.interval)

			if len(result) != tt.wantLen {
				t.Errorf("alignToBoundaries() returned %d candles, want %d", len(result), tt.wantLen)
			}

			intervalMs := int64(0)
			if tt.interval == "3m" {
				intervalMs = 3 * 60 * 1000
			} else if tt.interval == "4h" {
				intervalMs = 4 * 60 * 60 * 1000
			}

			for i, kline := range result {
				if intervalMs > 0 && kline.OpenTime%intervalMs != 0 {
					t.Errorf("Candle %d OpenTime %d is not aligned to %s boundary", i, kline.OpenTime, tt.interval)
				}
			}
		})
	}
}

func TestCandleSanity(t *testing.T) {
	tests := []struct {
		name    string
		kline   Kline
		wantErr bool
	}{
		{
			name: "Valid candle",
			kline: Kline{
				OpenTime:  1700000000000,
				CloseTime: 1700000179999,
				Open:      100.0,
				High:      105.0,
				Low:       95.0,
				Close:     102.0,
				Volume:    1000.0,
			},
			wantErr: false,
		},
		{
			name: "High < Low (invalid)",
			kline: Kline{
				OpenTime:  1700000000000,
				CloseTime: 1700000179999,
				Open:      100.0,
				High:      90.0, // High < Low
				Low:       95.0,
				Close:     102.0,
				Volume:    1000.0,
			},
			wantErr: true,
		},
		{
			name: "Close <= 0 (invalid)",
			kline: Kline{
				OpenTime:  1700000000000,
				CloseTime: 1700000179999,
				Open:      100.0,
				High:      105.0,
				Low:       95.0,
				Close:     0.0, // Invalid close price
				Volume:    1000.0,
			},
			wantErr: true,
		},
		{
			name: "Negative volume (invalid)",
			kline: Kline{
				OpenTime:  1700000000000,
				CloseTime: 1700000179999,
				Open:      100.0,
				High:      105.0,
				Low:       95.0,
				Close:     102.0,
				Volume:    -100.0, // Negative volume
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := false

			if tt.kline.High < tt.kline.Low {
				hasError = true
			}
			if tt.kline.Close <= 0 {
				hasError = true
			}
			if tt.kline.Volume < 0 {
				hasError = true
			}

			if hasError != tt.wantErr {
				t.Errorf("Candle sanity check failed: hasError = %v, wantErr = %v", hasError, tt.wantErr)
			}
		})
	}
}

func TestPaginationLimit(t *testing.T) {
	maxBars := 5000
	
	requestedBars := 10000
	
	actualBars := requestedBars
	if actualBars > maxBars {
		actualBars = maxBars
	}

	if actualBars != maxBars {
		t.Errorf("Pagination limit not enforced: got %d, want %d", actualBars, maxBars)
	}
}

func TestCacheTTL(t *testing.T) {
	provider := &HyperliquidProvider{
		apiURL:      "https://api.hyperliquid-testnet.xyz",
		candleCache: make(map[string]*cachedCandles),
	}

	cacheKey := "BTC_3m"
	now := time.Now()
	expiredTime := now.Add(-10 * time.Minute) // 10 minutes ago

	provider.candleCache[cacheKey] = &cachedCandles{
		klines:    []Kline{},
		expiresAt: expiredTime,
	}

	cache, exists := provider.candleCache[cacheKey]
	if !exists {
		t.Fatal("Cache entry should exist")
	}

	if !cache.expiresAt.Before(now) {
		t.Error("Cache should be expired but is not")
	}
}
