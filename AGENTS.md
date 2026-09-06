# lottery-stats-server (Go)

Go 1.25 lottery computation + scraper service. gRPC + REST gateway. Module path:
`github.com/lihai1/stat-tree-server` (repo dir is `lottery-stats-server` — name mismatch
is intentional, do not "fix" it).

## Build / test / proto

```bash
make build              # build binary
make run                # go run cmd/server/main.go
make test               # ginkgo + gomega, all tests
make integration-test   # needs PostgreSQL
make compose-test       # spins test DB via docker-compose-test.yml, runs tests
make proto              # regenerate Go + gateway + openapiv2 stubs into pkg/gen/
```

Proto source: `proto/lottery.proto` (local copy). Stubs generated to `pkg/gen/`.
The orchestrator repo's `make proto-go` runs this inside the container.

## Package layout

- `cmd/server/main.go` — entry point; loads config, starts gRPC + REST gateway.
- `internal/config/` — env-based config struct.
- `internal/database/` — pgxpool connection, sets `search_path=lottery`.
- `internal/lottery-tree/` — **core algorithm, do not modify unless explicitly required.**
  Files: `types.go`, `node.go`, `tree.go`, `lottery_archive.go`, `form_generator.go`.
  See `internal/lottery-tree/README.md` for algorithm docs.
- `internal/repository/` — `lottery_result_repository.go`, pgx data access.
- `internal/scraper/` — `pais_scraper.go`, downloads CSV from pais.co.il.
- `internal/seeder/` — `lottery_seeder.go`, seeds DB on first boot if empty; `prize_seeder.go`, populates `prize_amounts`.
- `internal/services/` — `lottery_service.go`, gRPC service impl; `simulate.go`, Simulate backtest logic; `lottery_manager.go`, LRU archive cache. Archive is cached per date window and shared across RPCs.
- `internal/server/` — `grpc.go` (gRPC server), `gateway.go` (REST gateway + swagger).
- `internal/middleware/` — `auth.go` (JWT validation), `logging.go`.
- `internal/startup/` — `server.go`, wiring + scraper cron scheduler.
- `internal/models/` — `lottery_result.go` domain model.
- `db-migration/` — Liquibase YAML changelogs (table: `lottery_results`).
- `pkg/gen/` — generated proto stubs (do not edit).

## gRPC service

`lottery.v1.LotteryService` (proto/lottery.proto:18):

- `HealthCheck`, `GenerateForm`, `GetStatistics`, `Analyze`, `Simulate`.

Both the gRPC server and the REST gateway validate JWTs using the same
`AuthMiddleware` instance (shared JWKS cache). The gRPC interceptor reads the
`authorization` metadata entry; the HTTP middleware reads the `Authorization`
header. When `AUTH_ENABLED=true`, all non-health requests require a valid
Keycloak JWT (defense-in-depth; Traefik also validates at edge).

REST gateway (via grpc-gateway, gateway.go):

- `GET /health` (open)
- `POST /api/generate/form`, `POST /api/generate/pares`, `POST /api/generate/analyze`, `POST /api/generate/simulate`
- `GET /swagger` (openapi JSON)

`/health` is always open. All other routes (both gRPC and HTTP) require JWT
when `AUTH_ENABLED=true`.

## Database

- Schema: `lottery` (set via `search_path`). Table: `lottery_results`.
- Library: `pgx/v5` via pgxpool. Context passed to all repo methods.
- Migrations: Liquibase YAML in `db-migration/` (NOT Flyway — that's the Java service).

## Scraper

- `internal/scraper/pais_scraper.go` — downloads CSV from pais.co.il (external site).
- Cron in `startup/server.go:startScraperScheduler`.
- `LOTTERY_SCRAPER_CRON` (default `0 3 * * *` = daily 03:00).
- Cron parser is a simple custom one: supports daily, hourly, 15-min only. For complex
  cron, integrate `robfig/cron` — do not extend the hand-rolled parser.
- On boot: if `LOTTERY_SEED_ON_BOOT=true` and table empty, seeds from `lotto.data`.
  Scraper failures are logged but do NOT stop startup.
- Scraper inserts only new draws via `InsertNewDraws` (ON CONFLICT DO NOTHING) instead
  of upserting the full CSV every run. Cache invalidation is range-scoped: only
  archive windows overlapping the affected draw dates are cleared.
- Prize backfill (`prize_seeder.go`) runs best-effort on startup and reports the
  affected date range so the manager can invalidate only overlapping windows.

## Config (env vars)

Server: `SERVER_PORT`, `SERVER_HOST`, `GRPC_PORT`, `GATEWAY_PORT`
DB: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `DB_SCHEMA`
Auth: `AUTH_ENABLED`, `KEYCLOAK_JWKS_URL`, `KEYCLOAK_ISSUER`, `KEYCLOAK_AUDIENCE`
  - Orchestrator dev compose sets `KEYCLOAK_ISSUER=""` (disabled — external URL varies by
    deployment) and `KEYCLOAK_AUDIENCE=statistiloto-ui`. Signature + audience are still
    validated. The ngrok override sets `KEYCLOAK_ISSUER` to the public tunnel URL.
Scraper: `LOTTERY_SCRAPER_CRON`, `LOTTERY_SEED_ON_BOOT`

See `.env.example`.

## Conventions

- Errors wrapped with `fmt.Errorf("...: %w", err)`.
- Logging: `log/slog` with JSON handler (initialized in `cmd/server/main.go`).
  Structured key-value args; `slog.Info`/`slog.Warn`/`slog.Error`. No `log.Printf`.
- Repository pattern: services = business logic, repositories = data access.
- Archive cached per date window via `LotteryManager` (LRU, max 8 entries, lazy
  construction, single-flight on concurrent misses). The archive (`LotteryArchive`)
  is immutable after construction and safe for concurrent read-only use.
- Default archive start date: `2004-02-12` (current Israeli Lotto format, numbers 1–37).
- `GetStatistics` rejects `form_type > 6` (the tree is built through depth 6). Response includes `total_draws_in_range` (number of draws in the requested date window).
- `Analyze` treats all supplied numbers as regular numbers — no trailing strong-number stripping.

## Gotchas

- Go module is `github.com/lihai1/stat-tree-server`; repo dir is `lottery-stats-server`.
  Imports use the module path, not the dir name.
- `pkg/gen/` is generated — never edit by hand. Run `make proto`.
- Scraper depends on pais.co.il being reachable; seeder fetches live, not from a local file.
- Lottery-tree package files `list.go` and `lottery_array.go` have been deleted.
  Use `LotteryArchive` + `FormGenerator` instead.
- `db-migration/` uses Liquibase (YAML), not Flyway. Don't confuse with Java service.
