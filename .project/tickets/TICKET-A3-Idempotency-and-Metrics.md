# TICKET‑A3 — Idempotenza & Metriche

**Priority:** 🔴 Critical  
**Branch:** `feature/idempotency-metrics` (base: `gm-integ`)  
**Goal:** Garantire **idempotenza** degli ordini (no doppi fill su retry) e aggiungere **osservabilità** (Prometheus + `/readyz`) per latenza, slippage, errori e rifiuti.

---

## Scope
- **Client Order ID** deterministico su **tutti** gli ordini (open/close/partial/TP/SL).  
- Retry‑safe: su errore transitorio, ri‑invia lo **stesso COID**; gestisci eventuale “duplicate accepted”.  
- `/metrics` Prometheus + `/readyz`.  
- Strumentazione minima ma utile in decisione/esecuzione.

### Out of scope
- Export complesso di tracing distribuito (fase successiva).

---

## Tasks

1) **COID deterministico**
   - Funzione `MakeCOID(traderID, symbol, action, side, tsBucket, nonce)` → string breve e unica (es. `nofx-{trader}-{sym}-{act}-{bucket}-{h}` con hash).  
   - Usare **lo stesso COID** su retry; se la venue risponde “duplicate”, trattare come **OK**.  
   - Applicare **anche** a `partial_close`, `update_stop_loss`, `update_take_profit`.

2) **Retry & duplicate handling**
   - Timeouts/EOF/5xx → retry con backoff; stesso COID.  
   - Se risposta “already exists” → **idempotent OK**; recuperare stato/fill via query ordini.

3) **Metriche Prometheus**
   - `ai_inference_latency_ms` (histogram)  
   - `decision_to_fill_ms`  
   - `slippage_bps` (summary)  
   - `order_error_total{exchange,action,reason}`  
   - `risk_gate_rejections_total{reason}` (dal ticket A2)  
   - `provider_*` (dal ticket A1)  
   - endpoint `/metrics` (se non già presente).

4) **/readyz**
   - Verifiche: connettività DB, ping HL (o semplice request di info), ping LLM (o token usage endpoint).  
   - Distinguere `/healthz` (process up) vs `/readyz` (dipendenze OK).

5) **Log “reasoned”**
   - Per ogni invio ordine: log di input/COID/risultato in forma **non sensibile** (no segreti, no payload LLM raw).

---

## Acceptance Criteria

- **AC1**: Lo stesso ordine inviato 2 volte (con errori transienti) produce **un solo fill** (duplicate riconosciuto).  
- **AC2**: `decision_to_fill_ms` popolata; `slippage_bps` disponibile; errori categorizzati in `order_error_total{...}`.  
- **AC3**: `/readyz` restituisce **200** solo se DB/HL/LLM rispondono.  
- **AC4**: Nessun secret in chiaro nei log o nelle metriche.  
- **AC5**: Test di integrazione per retry/duplicate/partial‑fill verdi.

---

## Test Plan

- **Unit**: generazione COID, collisioni improbabili, parsing reason.  
- **Integrazione**:  
  - Simula timeout → retry con stesso COID → no doppio fill;  
  - Simula partial‑fill e update;  
  - Verifica esposizione metriche.

---

## Files (indicativi)

- `trader/hyperliquid_trader.go` (COID + retry), `trader/auto_trader.go` (strumentazione)  
- `api/server.go` (`/readyz`, `/metrics` se non già), `metrics/` (nuovo)  
- `README.md`, `CHANGELOG.md`

---

## Rollback

- Disabilita gli exporter (env) e usa COID solo in log (se proprio servisse). Non rompe il flow.
