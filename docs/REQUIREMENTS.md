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
  The strong number is appended as the last element of each form.
- **FR-3** `GetStatistics` — calculate frequent number pairs/groups over an
  optional date window, returning the top `how_many` pairs with occurrence
  counts. `form_type` must be ≤ 6 (the cached tree is built through depth 6;
  larger group sizes are rejected).
- **FR-4** `Analyze` — evaluate user-selected numbers against historical
  winning draws, returning grouped frequency results (one entry per group
  size 1–6) and the archive size used. All numbers in the form are treated
  as regular numbers — no trailing number is stripped as a strong number.
- **FR-4a** `Simulate` — backtest a user's ticket (6/8/10/12-number systematic
  forms) against every historical draw in an optional date window. For N > 6,
  all C(N,6) combinations are played per draw. Cost is calculated from the
  actual combination count (no artificial minimum). Returns per-draw results
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
- **FR-5b** Prize backfill reports the affected date range so the archive
  cache can invalidate only overlapping windows (not global invalidation).
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

- **ALG-1** The core tree-based analysis engine preserves the original Java
  algorithm behavior (frequency-ranked tries, ReGroup for willBe, recursive
  backtracking). The implementation has been refactored from `LotteryArray`
  into `LotteryArchive` (immutable, cached) + `FormGenerator` (request-scoped
  mutable state), but the algorithm logic is preserved.
- **ALG-2** `FormGenerator.Generate` mirrors the Java
  `generateNewCombination(cResults, formType, willBe)` method:
  `RankNumbers` (rank by frequency via `CollectNodes`) → `regroup`
  (front-load willBe) → recursive backtracking → `addResult`.
- **ALG-3** Winning-combination rejection uses `HasWon`, an O(1) hash-set
  lookup on the archive's `WinningCombos` map, replacing the former O(m)
  `InTree` tree traversal.
- **ALG-4** The archive is cached per date window via `LotteryManager` (LRU,
  max 8 entries, lazy construction, single-flight on concurrent misses).
  Concurrent requests for the same window share one immutable `LotteryArchive`.
  Cache invalidation is range-scoped: only windows overlapping a changed draw
  date are evicted.
- **ALG-5** The default archive start date is `2004-02-12` (current Israeli
  Lotto format, numbers 1–37, strong 1–7) when the request does not specify a
  `from` date. Draws before this date used the old 6/49 format (regular numbers
  up to 49, no separate strong number) and are not comparable to the current
  game. A caller may explicitly request an earlier window — the service accepts
  it and builds the archive from whatever draws exist — but the results reflect
  the old format. The UI shows a disclaimer when the user selects a pre-2004
  start date. See `internal/lottery-tree/README.md` → "Historical Format &
  Archive Default" for the full format-era breakdown.
- **ALG-5a** The strong-number tree is sized to `maxStrong + 2` (slots for
  strong numbers 1–9). `TopStrong` only reads 1–7. Values outside 1–9 (from
  pre-2004 6/49 rows where field 9 is a 7th regular up to 49) are silently
  dropped by the tree's bounds check. This is intentional — those values are
  not strong numbers in the current game.
- **ALG-6** `GetStatistics` rejects `form_type > 6` — the tree is built
  through depth 6 and larger group sizes have no meaning.
- **ALG-7** `Analyze` treats all supplied numbers as regular numbers — no
  trailing number is stripped as a strong number.

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
- **SCR-3** Scraper inserts only new draws via `InsertNewDraws`
  (ON CONFLICT DO NOTHING) — existing draws are not upserted or overwritten.
- **SCR-4** Cache invalidation after scraper runs is range-scoped: only
  archive windows overlapping the affected draw dates are evicted, not the
  entire cache.

---

## Stateless Requirements

- **STATE-1** No in-process session state — any instance can serve any request.
- **STATE-2** Archive is cached per date window via `LotteryManager` (LRU,
  max 8 entries). The cache holds immutable `LotteryArchive` instances that
  are safe for concurrent read-only use. Mutable request-specific state
  (form generation tries, seen-set, results) is isolated in `FormGenerator`
  and never shared across requests.
- **STATE-3** JWT validation is stateless (JWKS cached but not session-based).
- **STATE-4** Horizontally scalable: `docker compose up --scale lottery=2`.
  Note: each instance maintains its own LRU cache (no distributed cache).

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
