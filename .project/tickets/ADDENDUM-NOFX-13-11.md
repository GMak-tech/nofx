# ADDENDUM‑NOFX‑13‑11.md (da applicare a tutti i ticket)

**Questa versione dei ticket è allineata allo stato NOFX del 13/11. I punti seguenti sono vincoli di integrazione.**

1) **Config & precedence**: oltre alla variabile globale `NOFX_DATA_PROVIDER`, aggiungere **override per‑trader (DB/UI)**. Precedenza: **per‑trader** → **ENV** → **default (AUTO)**.  
2) **Injection point**: iniettare il `DataProvider` in `AutoTrader.Initialize()`; *non* hard‑codare nel vecchio `market/data.go`.  
3) **Symbol mapping dinamico**: generare mapping **dal `meta.universe` di Hyperliquid** (cache con TTL, fallback sicuro). **No** tabelle hard‑coded.  
4) **Candle boundaries & pagination**: candele 3m/4h allineate ai **boundary UTC** HL, **niente barre parziali**, paginare (limite 5000 barre/call).  
5) **Caching**:  
   - Candele: TTL **fino al prossimo boundary**;  
   - `meta/asset ctx` (funding, OI, mark): TTL **5–15 s** configurabile;  
   - esporre `provider_cache_hit_ratio`.  
6) **Risk‑gates** applicati **prima** di `open_*` **e** coerenti con `partial_close / update_stop_loss / update_take_profit`.  
7) **PnL/kill‑switch**: usare **TotalEquity = WalletBalance + UnrealizedPnL** (vedi `docs/pnl.md`).  
8) **Idempotenza**: **Client Order ID** su **tutti** gli ordini (open/close/partial/TP/SL) con retry‑safe e gestione duplicate/partial‑fill.  
9) **Observability**: aggiungere `/readyz` (DB, HL, LLM) e metriche Prometheus:  
   - `ai_inference_latency_ms`, `decision_to_fill_ms`, `slippage_bps`,  
   - `order_error_total{exchange,action,reason}`, `risk_gate_rejections_total{reason}`,  
   - `provider_*` (*requests, errors, latency, cache_hit_ratio*).  
10) **Egress allow‑list (applicativa)**: consentire solo HL main/testnet, host LLM configurati e SMTP (se usato). Bloccare e loggare tutto il resto.  
11) **Rollback semplice**: `NOFX_DATA_PROVIDER=binance` per tornare al provider precedente senza modifiche di codice.
