package market

import (
	"testing"
	"time"
)

func TestSymbolMapping(t *testing.T) {
	mapper := &SymbolMapper{
		pairToCoin: make(map[string]string),
		coinToPair: make(map[string]string),
		lastUpdate: time.Time{},
		cacheTTL:   10 * time.Second,
	}

	mapper.pairToCoin["BTCUSDT"] = "BTC"
	mapper.pairToCoin["ETHUSDT"] = "ETH"
	mapper.pairToCoin["SOLUSDT"] = "SOL"
	mapper.coinToPair["BTC"] = "BTCUSDT"
	mapper.coinToPair["ETH"] = "ETHUSDT"
	mapper.coinToPair["SOL"] = "SOLUSDT"
	mapper.lastUpdate = time.Now()

	tests := []struct {
		name     string
		input    string
		method   string
		expected string
		wantErr  bool
	}{
		{
			name:     "PairToCoin: BTCUSDT -> BTC",
			input:    "BTCUSDT",
			method:   "PairToCoin",
			expected: "BTC",
			wantErr:  false,
		},
		{
			name:     "PairToCoin: ETHUSDT -> ETH",
			input:    "ETHUSDT",
			method:   "PairToCoin",
			expected: "ETH",
			wantErr:  false,
		},
		{
			name:     "CoinToPair: BTC -> BTCUSDT",
			input:    "BTC",
			method:   "CoinToPair",
			expected: "BTCUSDT",
			wantErr:  false,
		},
		{
			name:     "CoinToPair: ETH -> ETHUSDT",
			input:    "ETH",
			method:   "CoinToPair",
			expected: "ETHUSDT",
			wantErr:  false,
		},
		{
			name:     "PairToCoin: Unknown pair",
			input:    "XYZUSDT",
			method:   "PairToCoin",
			expected: "XYZ",
			wantErr:  false,
		},
		{
			name:     "CoinToPair: Unknown coin",
			input:    "XYZ",
			method:   "CoinToPair",
			expected: "XYZUSDT",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			var err error

			if tt.method == "PairToCoin" {
				result, err = mapper.PairToCoin(tt.input)
			} else {
				result, err = mapper.CoinToPair(tt.input)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("%s error = %v, wantErr %v", tt.method, err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != tt.expected {
				t.Errorf("%s(%s) = %v, want %v", tt.method, tt.input, result, tt.expected)
			}
		})
	}
}

func TestSymbolMapperCacheTTL(t *testing.T) {
	mapper := &SymbolMapper{
		pairToCoin: make(map[string]string),
		coinToPair: make(map[string]string),
		lastUpdate: time.Now().Add(-20 * time.Second), // Expired cache
		cacheTTL:   10 * time.Second,
	}

	if !mapper.lastUpdate.Add(mapper.cacheTTL).Before(time.Now()) {
		t.Error("Cache should be expired but is not")
	}
}

func TestGetAllCoins(t *testing.T) {
	mapper := &SymbolMapper{
		pairToCoin: make(map[string]string),
		coinToPair: make(map[string]string),
		lastUpdate: time.Now(),
		cacheTTL:   10 * time.Second,
	}

	mapper.coinToPair["BTC"] = "BTCUSDT"
	mapper.coinToPair["ETH"] = "ETHUSDT"
	mapper.coinToPair["SOL"] = "SOLUSDT"

	coins := mapper.GetAllCoins()

	if len(coins) != 3 {
		t.Errorf("GetAllCoins() returned %d coins, want 3", len(coins))
	}

	expectedCoins := map[string]bool{"BTC": true, "ETH": true, "SOL": true}
	for _, coin := range coins {
		if !expectedCoins[coin] {
			t.Errorf("Unexpected coin in result: %s", coin)
		}
		delete(expectedCoins, coin)
	}

	if len(expectedCoins) > 0 {
		t.Errorf("Missing coins in result: %v", expectedCoins)
	}
}
