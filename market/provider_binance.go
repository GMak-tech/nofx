package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

type BinanceProvider struct {
	baseURL string
}

func NewBinanceProvider() *BinanceProvider {
	return &BinanceProvider{
		baseURL: "https://fapi.binance.com/fapi/v1",
	}
}

func (p *BinanceProvider) Name() string {
	return "binance"
}

func (p *BinanceProvider) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("%s/klines?symbol=%s&interval=%s&limit=%d",
		p.baseURL, symbol, interval, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawData [][]interface{}
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	klines := make([]Kline, len(rawData))
	for i, item := range rawData {
		openTime := int64(item[0].(float64))
		open, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])
		closeTime := int64(item[6].(float64))

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

func (p *BinanceProvider) GetFundingRate(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/premiumIndex?symbol=%s", p.baseURL, symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}

func (p *BinanceProvider) GetOpenInterest(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/openInterest?symbol=%s", p.baseURL, symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)
	return oi, nil
}

func (p *BinanceProvider) GetMarkPrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/premiumIndex?symbol=%s", p.baseURL, symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		MarkPrice string `json:"markPrice"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	price, _ := strconv.ParseFloat(result.MarkPrice, 64)
	return price, nil
}

func (p *BinanceProvider) GetMarketData(ctx context.Context, symbol string) (*Data, error) {
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
	if len(klines3m) >= 21 { // Need at least 21 klines (current + 20 previous)
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
		Average: oi * 0.999, // Approximate average
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
