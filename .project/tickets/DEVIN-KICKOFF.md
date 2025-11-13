# NOFX — HL Readiness & Hardening (Devin Kick‑off)

**Repo (fork):** GMak-tech/nofx  
**Base branch:** `main` → work off a feature integration branch `gm-integ`

## Scope (3 PRs, in order)
1. **PR #1 — HL native Data Provider (AUTO + per‑trader override)**
2. **PR #2 — Risk Gates (minimal pre‑send)**
3. **PR #3 — Idempotency & Metrics (/readyz + Prometheus)**

> All features must be **flagged**, **rollback‑safe**, and **backward‑compatible** with Binance.

---

## Staging on EC2 (same host, separate ports)
- **Web**: `:3001` — **API**: `:8081` (restrict SG to my IP)
- **Base ENV**:
  - `NOFX_DATA_PROVIDER=AUTO`
  - Later (PR #2): `NOFX_RISK_GATES_ENABLED=true` (per‑trader overrides as needed)
  - Later (PR #3): `NOFX_METRICS_ENABLED=true`
- Keep **E2E encryption** as-is; **no secrets** in logs.

### Acceptance metrics (global)
- Venue drift (BTC/ETH, HL testnet): **median < 10 bps**
- p95 `GetMarketData("BTC","3m",100)` **< 500 ms** (warm cache)
- Provider error-rate **< 1%**
- Retry with same COID → **single fill** (idempotent)
- `/readyz` returns 200 only if **DB + HL + LLM** healthy
- Prometheus exports: `provider_*`, `risk_gate_rejections_total{reason}`, `ai_inference_latency_ms`, `decision_to_fill_ms`, `slippage_bps`, `order_error_total{...}`

---

## Workplan

### A) Repo tasks
- Create folder `.project/tickets/` and commit these 4 docs:
  - ADDENDUM-NOFX-13-11.md
  - TICKET-A1-HL-Data-Provider-AUTO.md
  - TICKET-A2-Risk-Gates-Min.md
  - TICKET-A3-Idempotency-and-Metrics.md
- Open **3 GitHub Issues** (A1/A2/A3). Prepend **Addendum** to each body.
  - Labels: `feat`, `hl`, `risk`, `observability`, `good-first-review`
  - Milestone: `HL-readiness`

### B) Branching
- `feature/hl-provider-auto`  → PR #1
- `feature/risk-gates-min`   → PR #2
- `feature/idempotency-metrics` → PR #3

### C) Staging
- Bring up a second stack on ports `3001/8081`.
- After each PR, run the **Test Plan** in the corresponding ticket and capture artifacts (logs/metrics).

### D) Upstream PRs (after fork merges)
- Rebase on `upstream/main` (NoFxAiOS/nofx).
- Open **3 small PRs**:
  1) `feat(provider): Hyperliquid native data provider (AUTO, per-trader override, rollback-safe)`
  2) `feat(risk): minimal pre-send risk gates (kill-switch, price-tolerance bps, confidence, margin cap)`
  3) `feat(obs): idempotent COID + Prometheus metrics + /readyz`
- Include: motivation, flags, tests (green), docs, rollback, screenshots/metrics snippets.

### E) Security
- Keep `registration_enabled=false` after admin bootstrap.
- Do not commit `config.db` or secrets.
- Enforce EC2 SG to my IP on 3001/8081.
- Grep logs for accidental secrets on each run.

---

## Optional (CLI helpers)

### Create issues with GitHub CLI
```bash
gh issue create -t "A1 — HL Native Data Provider (AUTO)" \
  -F .project/tickets/TICKET-A1-HL-Data-Provider-AUTO.md -l feat,hl,observability -m HL-readiness

gh issue create -t "A2 — Risk Gates (minimal pre-send)" \
  -F .project/tickets/TICKET-A2-Risk-Gates-Min.md -l feat,risk -m HL-readiness

gh issue create -t "A3 — Idempotency & Metrics (/readyz)" \
  -F .project/tickets/TICKET-A3-Idempotency-and-Metrics.md -l feat,observability -m HL-readiness
```

### Prometheus scrape example
```yaml
scrape_configs:
  - job_name: 'nofx-api'
    scrape_interval: 10s
    static_configs:
      - targets: ['YOUR_EC2_IP:8081']
```

---

## Deliverables per PR
- Code + unit/integration tests + README/.env.example/CHANGELOG updates
- Staging artifacts (drift/latency/metrics logs and screenshots)
- Clear rollback notes and feature flags
