package market

import (
	"os"
	"testing"
)

func TestNewDataProvider(t *testing.T) {
	tests := []struct {
		name           string
		envVar         string
		traderExchange string
		wantProvider   string
		wantErr        bool
	}{
		{
			name:           "AUTO mode with binance trader",
			envVar:         "AUTO",
			traderExchange: "binance",
			wantProvider:   "binance",
			wantErr:        false,
		},
		{
			name:           "AUTO mode with hyperliquid trader",
			envVar:         "AUTO",
			traderExchange: "hyperliquid",
			wantProvider:   "hyperliquid",
			wantErr:        false,
		},
		{
			name:           "Force binance provider",
			envVar:         "binance",
			traderExchange: "hyperliquid",
			wantProvider:   "binance",
			wantErr:        false,
		},
		{
			name:           "Force hyperliquid provider",
			envVar:         "hyperliquid",
			traderExchange: "binance",
			wantProvider:   "hyperliquid",
			wantErr:        false,
		},
		{
			name:           "Default to AUTO when env not set",
			envVar:         "",
			traderExchange: "binance",
			wantProvider:   "binance",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				os.Setenv("NOFX_DATA_PROVIDER", tt.envVar)
				defer os.Unsetenv("NOFX_DATA_PROVIDER")
			}

			provider, err := GetProviderForTrader(tt.traderExchange, nil, "", "")
			
			if (err != nil) != tt.wantErr {
				t.Errorf("GetProviderForTrader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && provider.Name() != tt.wantProvider {
				t.Errorf("GetProviderForTrader() provider = %v, want %v", provider.Name(), tt.wantProvider)
			}
		})
	}
}

func TestProviderPrecedence(t *testing.T) {
	os.Setenv("NOFX_DATA_PROVIDER", "binance")
	defer os.Unsetenv("NOFX_DATA_PROVIDER")

	provider, err := GetProviderForTrader("binance", nil, "hyperliquid", "")
	if err != nil {
		t.Fatalf("GetProviderForTrader() error = %v", err)
	}

	if provider.Name() != "hyperliquid" {
		t.Errorf("Expected per-trader override to take precedence, got %v", provider.Name())
	}
}
