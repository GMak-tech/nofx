package market

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sonirico/go-hyperliquid"
)

type SymbolMapper struct {
	exchange      *hyperliquid.Exchange
	coinToPair    map[string]string // BTC -> BTCUSDT
	pairToCoin    map[string]string // BTCUSDT -> BTC
	lastUpdate    time.Time
	cacheTTL      time.Duration
	mu            sync.RWMutex
}

func NewSymbolMapper(exchange *hyperliquid.Exchange, cacheTTL time.Duration) *SymbolMapper {
	if cacheTTL == 0 {
		cacheTTL = 10 * time.Second // Default 10s TTL
	}
	return &SymbolMapper{
		exchange:   exchange,
		coinToPair: make(map[string]string),
		pairToCoin: make(map[string]string),
		cacheTTL:   cacheTTL,
	}
}

func (sm *SymbolMapper) RefreshMapping(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	meta, err := sm.exchange.Info().Meta(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch HL meta: %w", err)
	}

	newCoinToPair := make(map[string]string)
	newPairToCoin := make(map[string]string)

	for _, asset := range meta.Universe {
		coin := asset.Name
		pair := coin + "USDT" // Standard pair format

		newCoinToPair[coin] = pair
		newPairToCoin[pair] = coin
	}

	sm.coinToPair = newCoinToPair
	sm.pairToCoin = newPairToCoin
	sm.lastUpdate = time.Now()

	log.Printf("✓ Symbol mapping refreshed: %d coins from HL universe", len(sm.coinToPair))
	return nil
}

func (sm *SymbolMapper) PairToCoin(pair string) (string, error) {
	sm.mu.RLock()
	needsRefresh := time.Since(sm.lastUpdate) > sm.cacheTTL
	sm.mu.RUnlock()

	if needsRefresh {
		if err := sm.RefreshMapping(context.Background()); err != nil {
			log.Printf("⚠️  Failed to refresh symbol mapping: %v", err)
		}
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if coin, ok := sm.pairToCoin[pair]; ok {
		return coin, nil
	}

	if strings.HasSuffix(pair, "USDT") {
		coin := strings.TrimSuffix(pair, "USDT")
		log.Printf("⚠️  Symbol %s not in mapping, using fallback: %s", pair, coin)
		return coin, nil
	}

	return "", fmt.Errorf("cannot map pair %s to HL coin", pair)
}

func (sm *SymbolMapper) CoinToPair(coin string) (string, error) {
	sm.mu.RLock()
	needsRefresh := time.Since(sm.lastUpdate) > sm.cacheTTL
	sm.mu.RUnlock()

	if needsRefresh {
		if err := sm.RefreshMapping(context.Background()); err != nil {
			log.Printf("⚠️  Failed to refresh symbol mapping: %v", err)
		}
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if pair, ok := sm.coinToPair[coin]; ok {
		return pair, nil
	}

	pair := coin + "USDT"
	log.Printf("⚠️  Coin %s not in mapping, using fallback: %s", coin, pair)
	return pair, nil
}

func (sm *SymbolMapper) GetAllCoins() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	coins := make([]string, 0, len(sm.coinToPair))
	for coin := range sm.coinToPair {
		coins = append(coins, coin)
	}
	return coins
}
