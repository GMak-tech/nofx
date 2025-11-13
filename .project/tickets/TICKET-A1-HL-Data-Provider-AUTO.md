# TICKET‑A1 — HL Native Data Provider (AUTO)

**Priority:** 🔴 Critical  
**Branch:** `feature/hl-provider-auto` (base: `gm-integ`)  
**Goal:** Eliminare *venue drift* fornendo **dati Hyperliquid nativi** quando il trader opera su HL. Se il trader opera su Binance, restare su Binance. Introdurre **AUTO** + override per‑trader.

---

## Scope
- Nuova interfaccia `market.DataProvider` e **factory** di selezione provider.  
- Implementazione `HyperliquidProvider` (candele 3m/4h, funding, OI, mark).  
- **AUTO mode** con precedence per‑trader → ENV → default.  
- Symbol mapping dinamico da `meta.universe` HL (cache).  
- Caching con TTL “intelligenti” + **metriche provider**.  
- Integrazione **non invasiva** (injection nel trader) e **rollback** immediato.

### Out of scope
- Modifiche alla UI (non necessarie).  
- Backtesting (ticket separato).

---

## Tasks

1) **Interfaccia & factory**
   - `market/provider.go`
     ```go
     type DataProvider interface {
       GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error)
       GetFundingRate(ctx context.Context, symbol string) (float64, error)
       GetOpenInterest(ctx context.Context, symbol string) (float64, error)
       GetMarkPrice(ctx context.Context, symbol string) (float64, error)
       GetMarketData(ctx context.Context, symbol string) (*MarketData, error)
     }

     type MarketData struct {
       Symbol       string
       LastPrice    float64
       FundingRate  float64
       OpenInterest float64
       Klines3m     []Kline
       Klines4h     []Kline
     }
     ```
   - `market/provider_factory.go`  
     Seleziona provider con precedence **per‑trader override → ENV `NOFX_DATA_PROVIDER` → AUTO**. In AUTO: `hyperliquid` se `trader.Exchange=="hyperliquid"`, altrimenti `binance`.

2) **HyperliquidProvider**
   - `market/provider_hyperliquid.go`  
     - **Re‑use** del client HL già presente in `trader/hyperliquid_trader.go` (estrarre un piccolo **InfoClient/facade** per non duplicare logica).  
     - **Candele 3m/4h**: allineare a boundary UTC, evitare barre parziali, **paginare** (≤5000 barre/call).  
     - **Funding/OI/Mark price**: da `meta/asset ctx`.  
     - **Cache**: candele TTL fino al prossimo boundary; ctx TTL 5–15 s (configurabile).  
     - **Metriche**: `provider_requests_total`, `provider_errors_total`, `provider_latency_seconds`, `provider_cache_hit_ratio`.

3) **Symbol mapping dinamico**
   - `market/symbol_map.go`  
     - Costruire mapping da **HL meta.universe** all’avvio (cache + TTL).  
     - Esempio: `BTCUSDT ↔ BTC`, `ETHUSDT ↔ ETH`.  
     - Fallback sicuro (se non trovato, usare `symbol` grezzo e loggare).

4) **Injection nel trader**
   - In `trader/auto_trader.go` (o nel costruttore): creare `at.dataProvider = provider_factory.New(...)`.  
   - **Rimpiazzare** le letture dirette di mercato con `at.dataProvider`.

5) **Docs & config**
   - `README.md`: sezione “Data Providers (AUTO/HL/Binance)” + rollback.  
   - `.env.example`: `NOFX_DATA_PROVIDER=AUTO`.

---

## Acceptance Criteria

- **AC1**: In modalità `AUTO`, un trader con `exchange=hyperliquid` usa **HL provider**; un trader su Binance usa **Binance provider**.  
- **AC2**: **Venue drift** medio < **10 bps** (misurato su BTC/ETH) tra dati del provider e mark di esecuzione.  
- **AC3**: p95 `GetMarketData("BTC","3m",100)` < **500 ms** (cache calda).  
- **AC4**: Error‑rate provider < **1%**; metriche `provider_*` esposte.  
- **AC5**: Nessuna regressione per trader Binance.  
- **AC6**: Rollback: `NOFX_DATA_PROVIDER=binance` ripristina il comportamento precedente senza code change.

---

## Test Plan

- **Unit**:  
  - rounding ai boundary 3m/4h,  
  - paginazione (limite 5000),  
  - mapping da universe HL (cache),  
  - sanity (High ≥ Low, Close > 0).  
- **Integrazione (testnet HL)**: backfill 30d 4h + 3d 3m, no panics; drift < 10 bps.  
- **Performance**: misurare latenza con cache calda; track `provider_cache_hit_ratio`.

---

## Files (indicativi)

- `market/provider.go`, `market/provider_factory.go`, `market/provider_hyperliquid.go`, `market/symbol_map.go`  
- piccoli cambi in `trader/auto_trader.go`, `decision/engine.go`  
- `README.md`, `.env.example`

---

## Rollback

- Set `NOFX_DATA_PROVIDER=binance` → riavvia → ritorno al feed Binance.
