# Stat-Tree Server — Requirements

Derived from the actual codebase (`go.mod`, `internal/`, `db-migration/`,
`Makefile`, `Dockerfile`, `docker-compose.yml`) and the
[README](../README.md).

---

## Functional Requirements

### gRPC / REST API
- **FR-1** `HealthCheck` — return service status, version, and
  `draws_loaded` count (number of historical draws in the archive).
- **FR-2** `GenerateForm` — generate `how_many` lottery number combinations
  using the tree-based algorithm (frequency-ranked tries, ReGroup for willBe,
  recursive backtracking for non-winning combos, strong-number selection).
- **FR-3** `GetStatistics` — calculate frequent number pairs/groups over an
  optional date window, returning the top `how_many` pairs with occurrence
  counts.
- **FR-4** `Analyze` — evaluate user-selected numbers against historical
  winning draws, returning grouped frequency results (one entry per group
  size 1–6) and the archive size used.
- **FR-4a** `Simulate` — backtest a user's ticket (6/8/10/12-number systematic
  forms) against every historical draw in an optional date window. For N > 6,
  all C(N,6) combinations are played per draw. Returns per-draw results
  (`SimulateDrawResult`: tier hits, prize won, ticket cost, `used_real_prizes`
  badge) and an aggregated `SimulateSummary` (total draws, combinations, spend,
  winnings, net, per-tier totals, draws priced with real scraped prizes).
  Prize amounts use scraped per-draw `prize_amounts` when available, falling
  back to service defaults or user-supplied overrides (`SimulateRequest.prize_amounts`).

### Scraper & Seeder
- **FR-5** Scheduled scraper fetches fresh lottery draws from the Israeli
  lottery site (pais.co.il) on a cron schedule (default: `0 3 * * *`).
- **FR-5a** Prize scraper populates the `prize_amounts` JSONB column on
  `lottery_results` (per-tier ILS prizes per draw, length-8 array). Used by
  `Simulate` for real per-draw prize data. Null when not yet fetched —
  `Simulate` falls back to defaults or user overrides.
- **FR-6** On first boot, if `lottery_results` is empty, seed from the
  `lotto.data` file (historical archive).
- **FR-7** Scraper is fault-tolerant — logs errors and preserves existing
  data on failure (seed remains as fallback).

### Authentication
- **FR-8** Validate Keycloak-issued JWTs (RS256) against the realm JWKS
  endpoint as defense-in-depth (Traefik also validates at the edge).
- **FR-9** `AUTH_ENABLED=false` disables JWT validation for local
  development and testing.

---

## Algorithm Requirements

- **ALG-1** The core `LotteryArray` tree-based analysis engine must preserve
  the original Java algorithm behavior — this is a port, not a redesign.
- **ALG-2** `GenerateNewCombinations` mirrors the Java
  `generateNewCombination(cResults, formType, willBe)` method:
  `SetTries` (rank by frequency) → `ReGroup` (front-load willBe) →
  recursive backtracking → `AddResult`.
- **ALG-3** `AllFormsCheck` and `NotInResults` guard conditions must match
  the original logic.
- **ALG-4** The archive is loaded fresh from the database per RPC call
  (stateless — no in-memory caching across requests).

---

## Data Requirements

- **DATA-1** The service owns only the `lottery` PostgreSQL schema.
- **DATA-2** Single table: `lottery_results` (id, draw_number, draw_date,
  numbers INTEGER[], strong INTEGER, lottery_type, prize_amounts JSONB
  (nullable, length-8 array of per-tier ILS prizes), created_at, updated_at).
- **DATA-3** No user data, no saved forms — identity is owned by Keycloak,
  user app data by the Java BFF.
- **DATA-4** Migrations managed by Liquibase, scoped to `currentSchema=lottery`.
- **DATA-5** Indexes on `draw_number` (unique), `draw_date`, `lottery_type`.

---

## Auth Requirements

- **AUTH-1** Keycloak JWKS cache with TTL (15 min), kid lookup, and
  double-checked locking refresh.
- **AUTH-2** Per-request token verification: signature (RS256), issuer,
  audience, expiry.
- **AUTH-3** JWKS fetch has a 10-second timeout with context cancellation.
- **AUTH-4** `AUTH_ENABLED` config flag toggles validation (default: true).

---

## Scraper Requirements

- **SCR-1** Cron schedule configurable via `LOTTERY_SCRAPER_CRON` env var
  (default: `0 3 * * *` — daily at 03:00).
- **SCR-2** `LOTTERY_SEED_ON_BOOT=true` seeds from `lotto.data` on first
  boot if the table is empty.
- **SCR-3** Scraper upserts (not overwrites) — existing draws are updated,
  new draws are inserted.

---

## Stateless Requirements

- **STATE-1** No in-process session state — any instance can serve any request.
- **STATE-2** Archive loaded per RPC from the database (no shared in-memory
  cache).
- **STATE-3** JWT validation is stateless (JWKS cached but not session-based).
- **STATE-4** Horizontally scalable: `docker compose up --scale lottery=2`.

---

## Testing Requirements

- **TEST-1** Ginkgo BDD framework with Gomega matchers.
- **TEST-2** Unit tests in `internal/` (service logic, algorithm, config).
- **TEST-3** Integration tests in `tests/integration/` (gRPC server, HTTP
  server) — require a running PostgreSQL.
- **TEST-4** `make test` runs all tests; `make test-coverage` adds coverage;
  `make test-race` adds race detection.
- **TEST-5** `make compose-test` starts PostgreSQL + runs tests via
  docker-compose-test.yml.

---

## Non-Functional Requirements

- **NFR-1** Go 1.25, gRPC + gRPC-Gateway (REST), pgx driver, Liquibase.
- **NFR-2** Multi-stage Docker build (golang:1.25-alpine builder → alpine runtime).
- **NFR-3** Ports: 8080 (REST gateway), 9090 (gRPC).
- **NFR-4** Environment-based configuration (`.env` file + env vars).
- **NFR-5** Proto stubs generated from shared `../proto/lottery.proto` via
  `make proto`.
