package market

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sonirico/go-hyperliquid"
)

type HyperliquidProvider struct {
	exchange     *hyperliquid.Exchange
	symbolMapper *SymbolMapper
	apiURL       string
	
	candleCache   map[string]*cachedCandles
	ctxCache      map[string]*cachedCtx
	cacheMu       sync.RWMutex
	ctxCacheTTL   time.Duration
	
	requestsTotal int64
	errorsTotal   int64
	cacheHits     int64
	cacheMisses   int64
	metricsMu     sync.RWMutex
}

type cachedCandles struct {
	klines    []Kline
	expiresAt time.Time
}

type cachedCtx struct {
	fundingRate  float64
	openInterest float64
	markPrice    float64
	expiresAt    time.Time
}

func NewHyperliquidProvider(exchange *hyperliquid.Exchange, apiURL string) *HyperliquidProvider {
	if apiURL == "" {
		apiURL = hyperliquid.MainnetAPIURL
	}
	
	provider := &HyperliquidProvider{
		exchange:      exchange,
		apiURL:        apiURL,
		candleCache:   make(map[string]*cachedCandles),
		ctxCache:      make(map[string]*cachedCtx),
		ctxCacheTTL:   10 * time.Second, // Default 10s TTL for ctx data
	}
	
	provider.symbolMapper = NewSymbolMapper(exchange, 10*time.Second)
	
	if err := provider.symbolMapper.RefreshMapping(context.Background()); err != nil {
		log.Printf("⚠️  Failed to initialize symbol mapping: %v", err)
	}
	
	return provider
}

func (p *HyperliquidProvider) Name() string {
	return "hyperliquid"
}

func (p *HyperliquidProvider) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error) {
	p.incrementRequests()
	
	coin, err := p.symbolMapper.PairToCoin(symbol)
	if err != nil {
		p.incrementErrors()
		return nil, fmt.Errorf("symbol mapping failed: %w", err)
	}
	
	cacheKey := fmt.Sprintf("%s:%s:%d", coin, interval, limit)
	if cached := p.getCachedCandles(cacheKey); cached != nil {
		p.incrementCacheHits()
		return cached, nil
	}
	p.incrementCacheMisses()
	
	klines, err := p.fetchKlinesFromAPI(ctx, coin, interval, limit)
	if err != nil {
		p.incrementErrors()
		return nil, err
	}
	
	klines = p.alignToBoundaries(klines, interval)
	
	expiresAt := p.calculateNextBoundary(interval)
	p.cacheCandles(cacheKey, klines, expiresAt)
	
	return klines, nil
}

func (p *HyperliquidProvider) fetchKlinesFromAPI(ctx context.Context, coin, interval string, limit int) ([]Kline, error) {
	hlInterval := interval
	
	endTime := time.Now().Unix() * 1000 // milliseconds
	
	var intervalMs int64
	switch interval {
	case "3m":
		intervalMs = 3 * 60 * 1000
	case "4h":
		intervalMs = 4 * 60 * 60 * 1000
	default:
		intervalMs = 60 * 1000 // Default 1m
	}
	
	startTime := endTime - (int64(limit) * intervalMs)
	
	url := fmt.Sprintf("%s/info", p.apiURL)
	
	reqBody := map[string]interface{}{
		"type": "candleSnapshot",
		"req": map[string]interface{}{
			"coin":      coin,
			"interval":  hlInterval,
			"startTime": startTime,
			"endTime":   endTime,
		},
	}
	
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var result []struct {
		T int64  `json:"t"` // timestamp (ms)
		O string `json:"o"` // open
		H string `json:"h"` // high
		L string `json:"l"` // low
		C string `json:"c"` // close
		V string `json:"v"` // volume
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	klines := make([]Kline, 0, len(result))
	for _, item := range result {
		open, _ := strconv.ParseFloat(item.O, 64)
		high, _ := strconv.ParseFloat(item.H, 64)
		low, _ := strconv.ParseFloat(item.L, 64)
		close, _ := strconv.ParseFloat(item.C, 64)
		volume, _ := strconv.ParseFloat(item.V, 64)
		
		klines = append(klines, Kline{
			OpenTime:  item.T,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: item.T + intervalMs - 1, // Approximate close time
		})
	}
	
	if len(klines) > limit {
		klines = klines[len(klines)-limit:]
	}
	
	return klines, nil
}

func (p *HyperliquidProvider) alignToBoundaries(klines []Kline, interval string) []Kline {
	if len(klines) == 0 {
		return klines
	}
	
	var intervalMs int64
	switch interval {
	case "3m":
		intervalMs = 3 * 60 * 1000
	case "4h":
		intervalMs = 4 * 60 * 60 * 1000
	default:
		return klines // No alignment for other intervals
	}
	
	now := time.Now().Unix() * 1000
	lastKline := klines[len(klines)-1]
	
	expectedCloseTime := (lastKline.OpenTime/intervalMs)*intervalMs + intervalMs
	
	if now < expectedCloseTime {
		klines = klines[:len(klines)-1]
	}
	
	return klines
}

func (p *HyperliquidProvider) calculateNextBoundary(interval string) time.Time {
	now := time.Now()
	
	switch interval {
	case "3m":
		minutes := now.Minute()
		nextMinute := ((minutes / 3) + 1) * 3
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), nextMinute, 0, 0, time.UTC)
	case "4h":
		hours := now.Hour()
		nextHour := ((hours / 4) + 1) * 4
		if nextHour >= 24 {
			return time.Date(now.Year(), now.Month(), now.Day()+1, nextHour-24, 0, 0, 0, time.UTC)
		}
		return time.Date(now.Year(), now.Month(), now.Day(), nextHour, 0, 0, 0, time.UTC)
	default:
		return now.Add(1 * time.Minute)
	}
}

func (p *HyperliquidProvider) GetFundingRate(ctx context.Context, symbol string) (float64, error) {
	p.incrementRequests()
	
	coin, err := p.symbolMapper.PairToCoin(symbol)
	if err != nil {
		p.incrementErrors()
		return 0, err
	}
	
	if cached := p.getCachedCtx(coin); cached != nil {
		p.incrementCacheHits()
		return cached.fundingRate, nil
	}
	p.incrementCacheMisses()
	
	log.Printf("⚠️  Funding rate not available from Hyperliquid meta.Universe, returning 0")
	
	p.cacheCtx(coin, 0, 0, 0)
	
	return 0, nil
}

func (p *HyperliquidProvider) GetOpenInterest(ctx context.Context, symbol string) (float64, error) {
	p.incrementRequests()
	
	coin, err := p.symbolMapper.PairToCoin(symbol)
	if err != nil {
		p.incrementErrors()
		return 0, err
	}
	
	if cached := p.getCachedCtx(coin); cached != nil {
		p.incrementCacheHits()
		return cached.openInterest, nil
	}
	p.incrementCacheMisses()
	
	log.Printf("⚠️  Open interest not available from Hyperliquid meta.Universe, returning 0")
	
	p.cacheCtx(coin, 0, 0, 0)
	
	return 0, nil
}

func (p *HyperliquidProvider) GetMarkPrice(ctx context.Context, symbol string) (float64, error) {
	p.incrementRequests()
	
	coin, err := p.symbolMapper.PairToCoin(symbol)
	if err != nil {
		p.incrementErrors()
		return 0, err
	}
	
	if cached := p.getCachedCtx(coin); cached != nil {
		p.incrementCacheHits()
		return cached.markPrice, nil
	}
	p.incrementCacheMisses()
	
	klines, err := p.GetKlines(ctx, symbol, "3m", 1)
	if err != nil || len(klines) == 0 {
		log.Printf("⚠️  Mark price not available, returning 0")
		p.cacheCtx(coin, 0, 0, 0)
		return 0, nil
	}
	
	markPrice := klines[0].Close
	p.cacheCtx(coin, 0, 0, markPrice)
	
	return markPrice, nil
}

func (p *HyperliquidProvider) GetMarketData(ctx context.Context, symbol string) (*Data, error) {
	symbol = Normalize(symbol)
	
	klines3m, err := p.GetKlines(ctx, symbol, "3m", 40)
	if err != nil {
		return nil, fmt.Errorf("failed to get 3m klines: %w", err)
	}
	
	klines4h, err := p.GetKlines(ctx, symbol, "4h", 60)
	if err != nil {
		return nil, fmt.Errorf("failed to get 4h klines: %w", err)
	}
	
	currentPrice := klines3m[len(klines3m)-1].Close
	currentEMA20 := CalculateEMA(klines3m, 20)
	currentMACD := CalculateMACD(klines3m)
	currentRSI7 := CalculateRSI(klines3m, 7)
	
	priceChange1h := 0.0
	if len(klines3m) >= 21 {
		price1hAgo := klines3m[len(klines3m)-21].Close
		if price1hAgo > 0 {
			priceChange1h = ((currentPrice - price1hAgo) / price1hAgo) * 100
		}
	}
	
	priceChange4h := 0.0
	if len(klines4h) >= 2 {
		price4hAgo := klines4h[len(klines4h)-2].Close
		if price4hAgo > 0 {
			priceChange4h = ((currentPrice - price4hAgo) / price4hAgo) * 100
		}
	}
	
	oi, err := p.GetOpenInterest(ctx, symbol)
	if err != nil {
		oi = 0
	}
	oiData := &OIData{
		Latest:  oi,
		Average: oi * 0.999,
	}
	
	fundingRate, _ := p.GetFundingRate(ctx, symbol)
	
	intradayData := CalculateIntradaySeries(klines3m)
	
	longerTermData := CalculateLongerTermData(klines4h)
	
	return &Data{
		Symbol:            symbol,
		CurrentPrice:      currentPrice,
		PriceChange1h:     priceChange1h,
		PriceChange4h:     priceChange4h,
		CurrentEMA20:      currentEMA20,
		CurrentMACD:       currentMACD,
		CurrentRSI7:       currentRSI7,
		OpenInterest:      oiData,
		FundingRate:       fundingRate,
		IntradaySeries:    intradayData,
		LongerTermContext: longerTermData,
	}, nil
}

func (p *HyperliquidProvider) getCachedCandles(key string) []Kline {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()
	
	if cached, ok := p.candleCache[key]; ok {
		if time.Now().Before(cached.expiresAt) {
			return cached.klines
		}
	}
	return nil
}

func (p *HyperliquidProvider) cacheCandles(key string, klines []Kline, expiresAt time.Time) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	
	p.candleCache[key] = &cachedCandles{
		klines:    klines,
		expiresAt: expiresAt,
	}
}

func (p *HyperliquidProvider) getCachedCtx(coin string) *cachedCtx {
	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()
	
	if cached, ok := p.ctxCache[coin]; ok {
		if time.Now().Before(cached.expiresAt) {
			return cached
		}
	}
	return nil
}

func (p *HyperliquidProvider) cacheCtx(coin string, fundingRate, openInterest, markPrice float64) {
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	
	p.ctxCache[coin] = &cachedCtx{
		fundingRate:  fundingRate,
		openInterest: openInterest,
		markPrice:    markPrice,
		expiresAt:    time.Now().Add(p.ctxCacheTTL),
	}
}

func (p *HyperliquidProvider) incrementRequests() {
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	p.requestsTotal++
}

func (p *HyperliquidProvider) incrementErrors() {
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	p.errorsTotal++
}

func (p *HyperliquidProvider) incrementCacheHits() {
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	p.cacheHits++
}

func (p *HyperliquidProvider) incrementCacheMisses() {
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	p.cacheMisses++
}

func (p *HyperliquidProvider) GetMetrics() ProviderMetrics {
	p.metricsMu.RLock()
	defer p.metricsMu.RUnlock()
	
	totalCacheAccess := p.cacheHits + p.cacheMisses
	cacheHitRatio := 0.0
	if totalCacheAccess > 0 {
		cacheHitRatio = float64(p.cacheHits) / float64(totalCacheAccess)
	}
	
	return ProviderMetrics{
		RequestsTotal:  p.requestsTotal,
		ErrorsTotal:    p.errorsTotal,
		CacheHitRatio:  cacheHitRatio,
	}
}
