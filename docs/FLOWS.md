# Stat-Tree Server — Flow Documentation

Mermaid diagrams for the main flows from the Go lottery service perspective.

---

## 1. gRPC Request Flow

```mermaid
sequenceDiagram
    participant BFF as Java BFF (gRPC client)
    participant GW as gRPC-Gateway (REST)
    participant SVC as LotteryService
    participant MGR as LotteryManager
    participant REPO as LotteryResultRepository
    participant DB as PostgreSQL (lottery schema)
    participant ARCH as LotteryArchive

    alt Via gRPC (from BFF)
        BFF->>SVC: GenerateForm(ctx, request)
    else Via REST (gRPC-Gateway, admin/direct)
        GW->>SVC: GenerateForm(ctx, request)
    end

    SVC->>MGR: Archive(ctx, dateWindow)
    alt Cache miss (lazy build)
        MGR->>REPO: GetByDateRange(ctx, from, to)
        REPO->>DB: SELECT numbers, strong FROM lottery.lottery_results WHERE draw_date BETWEEN ...
        DB-->>REPO: Rows
        REPO-->>MGR: []LotteryResult
        MGR->>ARCH: NewLotteryArchive(lottery, from, to, draws)
        ARCH->>ARCH: Build regular tree (depth 6) + strong tree + winning-combo hash set
        MGR-->>SVC: *LotteryArchive (cached, LRU max 8)
    else Cache hit
        MGR-->>SVC: *LotteryArchive (from cache)
    end

    alt GenerateForm
        SVC->>ARCH: NewFormGenerator(formType, mode)
        ARCH->>ARCH: RankNumbers(mode) via CollectNodes(1, mode)
        ARCH->>ARCH: TopStrong(mode)
        ARCH-->>SVC: FormGenerator
        SVC->>ARCH: gen.Generate(howMany, willBe) — backtracking + HasWon check
        ARCH-->>SVC: Results [][]int (with strong appended)
        SVC-->>BFF: GenerateFormResponse { forms: [...] }
    else GetStatistics
        SVC->>ARCH: TopGroups(howMany, formType, mode) via CollectNodes
        ARCH-->>SVC: []NodeEntry
        SVC-->>BFF: GetStatisticsResponse { pairs: [...] }
    else Analyze
        SVC->>ARCH: AnalyzeForm(form) — all numbers as regulars
        ARCH-->>SVC: AnalyzeResponse { frequency_groups, archive_size }
        SVC-->>BFF: AnalyzeResponse
    else Simulate
        SVC->>ARCH: Iterate draws, match combinations per draw
        ARCH-->>SVC: Per-draw results + summary
        SVC-->>BFF: SimulateResponse
    end
```

---

## 2. Algorithm Flow (FormGenerator.Generate)

```mermaid
flowchart TD
    START[Generate<br/>howMany, willBe] --> RANK[RankNumbers mode<br/>via CollectNodes 1, mode]
    RANK --> STRONG[TopStrong mode<br/>pick top strong number]
    STRONG --> INIT[Initialize tries, seen, results<br/>per-request mutable state]
    INIT --> LOOP{"i < howMany?"}
    LOOP -->|Yes| COPY[Copy ranked into tries<br/>restore frequency order]
    COPY --> REGROUP[regroup willBe<br/>front-load user's preferred numbers]
    REGROUP --> SEARCH[search<br/>recursive backtracking]
    SEARCH --> CHECK{"HasWon?<br/>AND not in seen?"}
    CHECK -->|Yes| ADD[addResult<br/>append form + strong to results]
    ADD --> NEXT[i++]
    NEXT --> LOOP
    CHECK -->|No| RECURSE[Try next position<br/>swap, recurse]
    RECURSE --> SEARCH
    RECURSE -->|Exhausted| BREAK[Break — no more valid combos]
    BREAK --> DONE
    LOOP -->|No| DONE["Return results<br/>[][]int with strong appended"]
```

---

## 3. Scraper Flow

```mermaid
sequenceDiagram
    participant CRON as Cron Scheduler
    participant SVC as Startup
    participant SCR as Scraper
    participant WEB as pais.co.il
    participant REPO as LotteryResultRepository
    participant DB as PostgreSQL
    participant MGR as LotteryManager

    CRON->>SVC: Trigger (LOTTERY_SCRAPER_CRON, default 0 3 * * *)
    SVC->>SCR: Run scraper

    SCR->>WEB: HTTP GET — fetch latest draws page
    WEB-->>SCR: HTML response
    SCR->>SCR: Parse draws (draw_number, draw_date, numbers[], strong)

    SCR->>REPO: InsertNewDraws(ctx, draws)
    REPO->>DB: INSERT ... ON CONFLICT (draw_number) DO NOTHING
    DB-->>REPO: Rows inserted (new only), affected date range
    REPO-->>SCR: (count, minDate, maxDate)

    SCR-->>SVC: Result (n new draws, affected date range)
    SVC->>MGR: InvalidateRange(minDate, maxDate)
    MGR->>MGR: Evict only cache windows overlapping the range
    SVC->>SVC: Log result

    alt First boot — table empty AND LOTTERY_SEED_ON_BOOT=true
        SVC->>SCR: Seed from /seed/lotto.data
        SCR->>SCR: Parse lotto.data (historical archive)
        SCR->>REPO: CreateBatch
        REPO->>DB: INSERT INTO lottery.lottery_results ...
        DB-->>REPO: Rows inserted
    end

    alt Prize backfill (best-effort, non-fatal)
        SVC->>REPO: GetDrawsWithoutPrizeRefs(limit)
        REPO-->>SVC: []DrawRef (draw_number, draw_date)
        SVC->>SCR: Scrape prize tables for each draw
        SCR->>WEB: HTTP GET per-draw prize page
        WEB-->>SCR: HTML prize table
        SCR->>REPO: UpdatePrizeAmounts(ctx, drawNumber, amounts)
        REPO-->>SVC: Affected draw date
        SVC->>MGR: InvalidateRange(affected dates)
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

---

## 6. Tree Algorithm Flow

The `LotteryArchive` owns two immutable prefix trees built once per date
window: a regular-number tree (depth `maxGroupSize = 6`) and a strong-number
tree (depth 1). Every statistics, analyze, and generation query reads from
these trees without mutating them.

> **Historical format note:** The default archive start date is `2004-02-12`,
> when the current Lotto format began (regular numbers 1–37, strong 1–7).
> Draws before that date used the old 6/49 format (regular numbers up to 49,
> no separate strong number) and are not comparable. The UI shows a disclaimer
> when the user selects a pre-2004 start date. See
> [`internal/lottery-tree/README.md`](../internal/lottery-tree/README.md#historical-format--archive-default)
> for the full format-era breakdown and strong-tree sizing rationale.

### 6.1 Build — `NewLotteryArchive` → `LoTree.Build` → `LoNode.Build`

```mermaid
flowchart TD
    START["NewLotteryArchive(lottery, from, to, draws)"] --> NUMS["Split draws into Numbers[][] and Strongs[][]"]
    NUMS --> COMBO["For each 6-number draw:<br/>sort, ComboMask, add to WinningCombos"]
    COMBO --> REG["NewLoTree(MaxNumber)<br/>Anchor = root node, Num=0"]
    COMBO --> STR["NewLoTree(maxStrong+2=9)<br/>Anchor = root node, Num=0"]

    REG --> RBUILD["Tree.Build(Numbers, maxGroupSize=6)"]
    STR --> SBUILD["StrongTree.Build(Strongs, 1)"]

    RBUILD --> RLOOP{"For each draw row"}
    RLOOP -->|next row| ANCHOR["Anchor.Build(form, 0, 6)"]
    ANCHOR --> NLOOP{"For i from current..<br/>len(form)?"}
    NLOOP -->|Yes| IDX["treeIndex = form[i] - n.Num - 1"]
    IDX --> BOUNDS{"0 <= treeIndex < len(n.Next)?"}
    BOUNDS -->|No| NLOOP
    BOUNDS -->|Yes| EXISTS{"n.Next[treeIndex] == nil?"}
    EXISTS -->|Yes| CREATE["NewLoNodeWithParent(n, form[i], ...)<br/>Count = 1"]
    EXISTS -->|No| INC["n.Next[treeIndex].Count++"]
    CREATE --> RECURSE["child.Build(form, i+1, howMany-1)"]
    INC --> RECURSE
    RECURSE --> DEPTH{"howMany == 0?"}
    DEPTH -->|No| NLOOP
    DEPTH -->|Yes| NLOOP
    NLOOP -->|No| RLOOP
    RLOOP -->|done| DONE["Archive ready:<br/>Tree + StrongTree + WinningCombos"]

    SBUILD --> SDONE["StrongTree: depth-1 nodes<br/>hold per-strong-number counts"]
    SDONE --> DONE
```

### 6.2 Query — `CollectNodes(size, mode)`

A single read-only walk that materialises every group of the requested size
together with its occurrence count, then sorts once. Replaces the former
`BestPares`, which re-traversed per group and mutated tree state.

```mermaid
flowchart TD
    CALL["CollectNodes(size, mode)"] --> GUARD{"size <= 0<br/>or Anchor == nil?"}
    GUARD -->|Yes| NIL["return nil"]
    GUARD -->|No| INIT["entries = []NodeEntry"]
    INIT --> CLOOP{"For each child of Anchor.Next"}
    CLOOP -->|next child| SKIP{"child == nil?"}
    SKIP -->|Yes| CLOOP
    SKIP -->|No| RECURSE["child.collectNodes(size-1, [], &entries)"]
    RECURSE --> CRECURSE["collectNodes(remaining, prefix, out)"]
    CRECURSE --> COPY["numbers = copy(prefix) + n.Num"]
    COPY --> BASE{"remaining == 0?"}
    BASE -->|Yes| EMIT["append NodeEntry{numbers, n.Count}"]
    EMIT --> CLOOP
    BASE -->|No| CHILDLOOP{"For each child of n.Next"}
    CHILDLOOP -->|next child| CSKIP{"child == nil?"}
    CSKIP -->|Yes| CHILDLOOP
    CSKIP -->|No| CRECURSE2["child.collectNodes(remaining-1, numbers, out)"]
    CRECURSE2 --> CHILDLOOP
    CHILDLOOP -->|done| CLOOP
    CLOOP -->|done| SORT["sort.SliceStable(entries)"]
    SORT --> MODE{"mode == weak?"}
    MODE -->|Yes| ASC["ascending by Count"]
    MODE -->|No| DESC["descending by Count"]
    ASC --> RET["return entries"]
    DESC --> RET
```

### 6.3 Analyze — `AnalyzeArray` → `analyzeBuild`

Walks only the paths that match the user-submitted form, appending one
`FrequencyEntry` per visited node directly into the matching `FrequencyGroup`.
All submitted values are treated as regular numbers; nothing is stripped.

```mermaid
flowchart TD
    CALL["AnalyzeArray(form, maxSize=6)"] --> GROUPS["Allocate 6 FrequencyGroups<br/>Size 1..6"]
    GROUPS --> ROOT["Anchor.analyzeBuild(form, 0, 0, 6, groups, nil)"]
    ROOT --> GUARD{"size == maxSize?"}
    GUARD -->|Yes| RETURN_UP["return (depth limit reached)"]
    GUARD -->|No| LOOP{"For i from current..<br/>len(form)?"}
    LOOP -->|Yes| IDX["treeIndex = form[i] - n.Num - 1"]
    IDX --> CHECK{"child exists<br/>and in bounds?"}
    CHECK -->|No| LOOP
    CHECK -->|Yes| ENTRY["numbers = copy(prefix) + form[i]"]
    ENTRY --> APPEND["append FrequencyEntry{numbers, child.Count}<br/>to groups[size]"]
    APPEND --> RECURSE["child.analyzeBuild(form, i+1, size+1, maxSize, groups, numbers)"]
    RECURSE --> GUARD
    LOOP -->|No| SORTGROUPS["Sort each group's entries<br/>by Count descending"]
    SORTGROUPS --> RESP["return AnalyzeResponse{<br/>FrequencyGroups, ArchiveSize}"]
```

### 6.4 Read APIs that reuse the tree

```mermaid
flowchart LR
    ARCH["LotteryArchive<br/>(immutable, cached)"] --> RN["RankNumbers(mode)<br/>= CollectNodes(1, mode)<br/>flattened to []int"]
    ARCH --> TG["TopGroups(howMany, size, mode)<br/>= CollectNodes(size, mode)<br/>truncated to howMany"]
    ARCH --> TS["TopStrong(mode)<br/>walks StrongTree depth-1 nodes<br/>picks max/min count"]
    ARCH --> AF["AnalyzeForm(form)<br/>= AnalyzeArray(form, 6)"]
    ARCH --> HW["HasWon(combo)<br/>WinningCombos map lookup O(1)"]

    RN --> GEN["FormGenerator.Generate"]
    TS --> GEN
    HW --> GEN
    TG --> STATS["GetStatistics RPC"]
    AF --> ANALYZE["Analyze RPC"]
    HW --> SIM["Simulate RPC<br/>per-draw combination match"]
```
