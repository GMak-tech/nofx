package market

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sonirico/go-hyperliquid"
)

type ProviderConfig struct {
	TraderOverride string
	
	TraderExchange string
	
	HLExchange *hyperliquid.Exchange
	HLAPIURL   string
}

func NewDataProvider(config ProviderConfig) (DataProvider, error) {
	providerName := determineProviderName(config)
	
	log.Printf("📊 Data Provider: %s (trader exchange: %s)", providerName, config.TraderExchange)
	
	switch strings.ToLower(providerName) {
	case "binance":
		return NewBinanceProvider(), nil
		
	case "hyperliquid", "hl":
		if config.HLExchange == nil {
			return nil, fmt.Errorf("Hyperliquid exchange client required for HL provider")
		}
		return NewHyperliquidProvider(config.HLExchange, config.HLAPIURL), nil
		
	case "auto":
		return selectAutoProvider(config)
		
	default:
		return nil, fmt.Errorf("unknown data provider: %s", providerName)
	}
}

func determineProviderName(config ProviderConfig) string {
	if config.TraderOverride != "" {
		log.Printf("  → Using per-trader override: %s", config.TraderOverride)
		return config.TraderOverride
	}
	
	if envProvider := os.Getenv("NOFX_DATA_PROVIDER"); envProvider != "" {
		log.Printf("  → Using ENV NOFX_DATA_PROVIDER: %s", envProvider)
		return envProvider
	}
	
	log.Printf("  → Using default AUTO mode")
	return "auto"
}

func selectAutoProvider(config ProviderConfig) (DataProvider, error) {
	exchange := strings.ToLower(config.TraderExchange)
	
	switch exchange {
	case "hyperliquid":
		log.Printf("  → AUTO selected: Hyperliquid provider (trader uses Hyperliquid)")
		if config.HLExchange == nil {
			return nil, fmt.Errorf("Hyperliquid exchange client required for HL trader")
		}
		return NewHyperliquidProvider(config.HLExchange, config.HLAPIURL), nil
		
	case "binance":
		log.Printf("  → AUTO selected: Binance provider (trader uses Binance)")
		return NewBinanceProvider(), nil
		
	case "aster":
		log.Printf("  → AUTO selected: Binance provider (trader uses Aster, fallback to Binance data)")
		return NewBinanceProvider(), nil
		
	default:
		log.Printf("  → AUTO selected: Binance provider (unknown exchange: %s, fallback to Binance)", exchange)
		return NewBinanceProvider(), nil
	}
}

func GetProviderForTrader(traderExchange string, hlExchange *hyperliquid.Exchange, hlAPIURL string, traderOverride string) (DataProvider, error) {
	config := ProviderConfig{
		TraderOverride: traderOverride,
		TraderExchange: traderExchange,
		HLExchange:     hlExchange,
		HLAPIURL:       hlAPIURL,
	}
	
	return NewDataProvider(config)
}
