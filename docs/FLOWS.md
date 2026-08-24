# Stat-Tree Server — Flow Documentation

Mermaid diagrams for the main flows from the Go lottery service perspective.

---

## 1. gRPC Request Flow

```mermaid
sequenceDiagram
    participant BFF as Java BFF (gRPC client)
    participant GW as gRPC-Gateway (REST)
    participant SVC as LotteryService
    participant REPO as LotteryResultRepository
    participant DB as PostgreSQL (lottery schema)
    participant LA as LotteryArray

    alt Via gRPC (from BFF)
        BFF->>SVC: GenerateForm(ctx, request)
    else Via REST (gRPC-Gateway, admin/direct)
        GW->>SVC: GenerateForm(ctx, request)
    end

    SVC->>SVC: loadArchive(ctx, from, to)
    SVC->>REPO: GetByDateRange(ctx, from, to)
    REPO->>DB: SELECT numbers, strong FROM lottery.lottery_results WHERE draw_date BETWEEN ...
    DB-->>REPO: Rows
    REPO-->>SVC: []LotteryResult
    SVC->>LA: NewLotteryArray() → populate Archive, StrongArchive, Strongs

    alt GenerateForm
        SVC->>LA: GenerateNewCombinations(howMany, formType, willBe)
        LA->>LA: BuildTree(6) → SetTries → ReGroup → generateNewCombination (backtracking)
        LA-->>SVC: Results [][]int
        SVC-->>BFF: GenerateFormResponse { forms: [...] }
    else GetStatistics
        SVC->>LA: Compute pairs/groups frequency
        LA-->>SVC: Pairs with counts
        SVC-->>BFF: GetStatisticsResponse { pairs: [...] }
    else Analyze
        SVC->>LA: Evaluate user form against archive
        LA-->>SVC: Frequency groups + archive size
        SVC-->>BFF: AnalyzeResponse { frequency_groups, archive_size }
    end
```

---

## 2. Algorithm Flow (GenerateNewCombinations)

```mermaid
flowchart TD
    START[GenerateNewCombinations<br/>cResults, formType, willBe] --> BUILD[BuildTree 6<br/>construct frequency trie]
    BUILD --> CONSIST[Consistance formType<br/>set form type constraints]
    CONSIST --> INIT[Initialize Results array<br/>cResults slots = nil]
    INIT --> LOOP{"i < cResults?"}
    LOOP -->|Yes| SETTRIES[SetTries<br/>rank numbers by frequency]
    SETTRIES --> REGROUP[ReGroup willBe<br/>front-load user's preferred numbers]
    REGROUP --> GEN[generateNewCombination<br/>recursive backtracking]
    GEN --> CHECK{"NotInResults<br/>AND AllFormsCheck?"}
    CHECK -->|Yes| CUT[CutArrayTo Tries, FormType<br/>extract combination]
    CUT --> ADD[AddResult<br/>store in Results[i]]
    ADD --> NEXT[i++]
    NEXT --> LOOP
    CHECK -->|No| RECURSE[Try next position<br/>swap, recurse]
    RECURSE --> GEN
    RECURSE -->|Exhausted| FALLBACK[Return nil — no valid combo found]
    FALLBACK --> NEXT
    LOOP -->|No| DONE[Return Results<br/>[][]int of cResults combinations]
```

---

## 3. Scraper Flow

```mermaid
sequenceDiagram
    participant CRON as Cron Scheduler
    participant SVC as LotteryService
    participant SCR as Scraper
    participant WEB as pais.co.il
    participant REPO as LotteryResultRepository
    participant DB as PostgreSQL

    CRON->>SVC: Trigger (LOTTERY_SCRAPER_CRON, default 0 3 * * *)
    SVC->>SCR: Run scraper

    SCR->>WEB: HTTP GET — fetch latest draws page
    WEB-->>SCR: HTML response
    SCR->>SCR: Parse draws (draw_number, draw_date, numbers[], strong)

    loop Each parsed draw
        SCR->>REPO: Upsert(ctx, draw)
        REPO->>DB: INSERT ... ON CONFLICT (draw_number) DO UPDATE
        DB-->>REPO: Row inserted/updated
    end

    SCR-->>SVC: Result (n draws upserted, errors if any)
    SVC->>SVC: Log result

    alt First boot — table empty AND LOTTERY_SEED_ON_BOOT=true
        SVC->>SCR: Seed from /seed/lotto.data
        SCR->>SCR: Parse lotto.data (historical archive)
        SCR->>REPO: Bulk insert
        REPO->>DB: INSERT INTO lottery.lottery_results ...
        DB-->>REPO: Rows inserted
    end
```

---

## 4. Auth Validation Flow

```mermaid
sequenceDiagram
    participant CLI as Client (BFF / REST)
    participant MW as AuthMiddleware
    participant JWKS as JWKS Cache
    participant KC as Keycloak JWKS Endpoint

    CLI->>MW: Request with Authorization: Bearer <token>
    MW->>MW: Extract token from header
    MW->>MW: Parse JWT header → kid

    MW->>JWKS: get(kid)
    alt Cache hit (TTL < 15 min)
        JWKS-->>MW: RSA public key (cached)
    else Cache miss / stale
        JWKS->>KC: GET /realms/statistiloto/protocol/openid-connect/certs
        KC-->>JWKS: JWKS JSON { keys: [...] }
        JWKS->>JWKS: Parse keys → RSA public keys, update cache
        JWKS-->>MW: RSA public key for kid
    end

    MW->>MW: Verify JWT signature (RS256)
    MW->>MW: Validate issuer (KEYCLOAK_ISSUER)
    MW->>MW: Validate audience (KEYCLOAK_AUDIENCE)
    MW->>MW: Validate expiry

    alt Valid
        MW-->>CLI: Forward request with user context
    else Invalid
        MW-->>CLI: 401 Unauthorized
    end
```

---

## 5. Startup Flow

```mermaid
flowchart TD
    START[main.go] --> CFG[Load Config<br/>env vars + .env file]
    CFG --> DB[Connect to PostgreSQL<br/>pgx pool, schema=lottery]
    DB --> LIQ[Liquibase migrations<br/>db-migration/migrations/]
    LIQ --> SEED{"LOTTERY_SEED_ON_BOOT<br/>AND table empty?"}
    SEED -->|Yes| SEEDRUN[Seed from lotto.data]
    SEED -->|No| SKIP[Skip seed]
    SEEDRUN --> SCRAPER
    SKIP --> SCRAPER

    SCRAPER[Start scraper scheduler<br/>cron: LOTTERY_SCRAPER_CRON] --> AUTH{AUTH_ENABLED?}
    AUTH -->|true| AUTHINIT[Initialize AuthMiddleware<br/>JWKS cache, Keycloak config]
    AUTH -->|false| AUTHSKIP[Skip JWT validation]
    AUTHINIT --> GRPC
    AUTHSKIP --> GRPC

    GRPC[Start gRPC server<br/>:9090] --> GW[Start gRPC-Gateway<br/>:8080 REST]
    GW --> HEALTH[Register health endpoint<br/>GET /health]
    HEALTH --> READY[Service ready<br/>serving gRPC + REST]
```
