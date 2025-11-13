# TICKET‑A2 — Risk‑Gates Minimi (pre‑send)

**Priority:** 🔴 Critical  
**Branch:** `feature/risk-gates-min` (base: `gm-integ`)  
**Goal:** Inserire **controlli di rischio pre‑invio** **semplici ma efficaci**, applicati *prima* delle azioni dell’executor (inclusi `partial_close`/`update_*`).

---

## Scope
- Kill‑switch giornaliero su **TotalEquity**.  
- Price‑deviation gate in **bps** rispetto a **mark price**.  
- Confidence gate (minimo).  
- Margin usage cap.  
- Log d’audit con reason codes + **metriche** `risk_gate_rejections_total{reason}`.  
- Configurazione per‑trader (DB/UI) e **flag globale** di abilito/disabilito.

### Out of scope
- Regole complesse (VaR, correlazioni, ATR sizing) → altra fase.

---

## Tasks

1) **Schema config per‑trader**
   - Campi (indicativi):  
     - `risk_gates_enabled` (bool),  
     - `max_daily_loss_pct` (es. 5.0),  
     - `price_tolerance_bps` (es. 50),  
     - `min_confidence` (es. 0.60),  
     - `max_margin_usage_pct` (es. 90.0),  
     - `anti_flip_cooldown_sec` (facoltativo).  
   - Persistenza in DB e UI (se già presente un pannello avanzato, usare lo stesso pattern).

2) **Pre‑send check**
   - In `trader/auto_trader.go` (o middleware dedicato), **prima** di `open_*` e, dove sensato, su `partial_close / update_stop_loss / update_take_profit`:  
     - **Kill‑switch**: calcolare drawdown giornaliero come `(DailyStartTotalEquity − CurrentTotalEquity) / DailyStartTotalEquity`. Se > `max_daily_loss_pct` → **PAUSE** per N min (config) + **close** posizioni opzionale.  
     - **Price‑deviation**: se `abs(decision.Entry − mark) / mark * 10_000 > price_tolerance_bps` → **REJECT** (o clamp se previsto).  
     - **Confidence**: `decision.confidence < min_confidence` → **HOLD**.  
     - **Margin usage**: `marginUsedPct > max_margin_usage_pct` → bloccare nuove aperture.

3) **Audit & metriche**
   - Log con **reason code** (es. `KILL_SWITCH`, `PRICE_TOLERANCE`, `CONFIDENCE_LOW`, `MARGIN_USAGE`).  
   - `risk_gate_rejections_total{reason,trader_id}`.

4) **Soft‑reload**
   - Agganciarsi al meccanismo di **reload** del trader quando cambiano i parametri risk in DB.

---

## Acceptance Criteria

- **AC1**: Con `risk_gates_enabled=true`, un ordine che supera **price_tolerance_bps** viene **rifiutato** e tracciato in metrica.  
- **AC2**: Kill‑switch scatta quando la perdita giornaliera su **TotalEquity** supera la soglia.  
- **AC3**: **Confidence** sotto soglia → **HOLD** (nessun ordine).  
- **AC4**: **Margin usage** sopra soglia → niente nuove aperture.  
- **AC5**: Reason codes presenti nei log e in `risk_gate_rejections_total`.  
- **AC6**: Disabilitando `risk_gates_enabled` il comportamento torna identico al pre‑feature.

---

## Test Plan

- **Unit**: ogni regola con soglie sopra/sotto, inclusi edge case.  
- **Scenario**: kill‑switch che scatta e “cooldown” rispettato; price‑deviation in bps; confidence gating.  
- **Replay smoke**: 1–2 ore di 3m/4h senza invio ordini reali per validare gating.

---

## Files (indicativi)

- `trader/auto_trader.go` (hook pre‑send), `risk/gates.go` (nuovo), `config/database.go` (campi), `api/server.go` (expose)  
- `README.md` (parametri), `CHANGELOG.md`

---

## Rollback

- Flag globale `NOFX_RISK_GATES_ENABLED=false` o `risk_gates_enabled=false` per‑trader → disattiva immediatamente i gates.
