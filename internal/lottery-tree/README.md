# Lottery Tree Package

A Go package for lottery number analysis and generation using tree-based data structures. This package provides algorithms for analyzing historical lottery data, generating number combinations, and identifying patterns in lottery results.

## Overview

The lottery-tree package implements a tree-based approach to lottery analysis, originally ported from Java. It uses a specialized tree structure (`LoTree`) to store and analyze lottery number sequences, enabling efficient pattern recognition and statistical analysis.

## Features

- **Tree-based number analysis**: Store and query lottery number sequences in a tree structure
- **Pattern detection**: Identify frequently occurring number pairs and sequences
- **Number generation**: Generate lottery combinations based on different strategies (frequent, less frequent, random)
- **Statistical analysis**: Calculate frequencies, pairs, and patterns from historical data
- **Multiple lottery types**: Support for different lottery games (Lottery, 777, 123)

## Package Structure

```
lottery-tree/
├── types.go            # Type definitions (Common, LotteryType)
├── node.go             # LoNode - tree node implementation
├── tree.go             # LoTree - tree structure, CollectNodes, NodeEntry
├── lottery_archive.go  # LotteryArchive - immutable cached archive + queries
├── form_generator.go   # FormGenerator - request-scoped generation state
└── README.md           # This file
```

## Core Components

### Types (`types.go`)

Defines the fundamental type constants used throughout the package:

- **Common**: Strength type for analysis
  - `Strong`: High-frequency numbers
  - `Weak`: Low-frequency numbers
  - `Random`: Random selection

- **LotteryType**: Supported lottery games
  - `Lottery`: Standard lottery (6/37)
  - `TripleSeven`: 777 game (7/70)
  - `OneTwoThree`: 123 game (3/10)

### LoNode (`node.go`)

Represents a node in the lottery tree structure.

**Fields:**
- `Piece []int`: The lottery form stored in this node
- `Prev *LoNode`: Reference to parent node
- `Next []*LoNode`: Child nodes
- `Num int`: The number value at this node
- `Count int`: Frequency count

**Key Methods:**
- `NewLoNode(n, lottery)`: Create a new node
- `NewLoNodeWithParent(parent, n, lottery)`: Create a node linked to a parent
- `Build(form, current, howMany)`: Build tree structure from a form

### LoTree (`tree.go`)

The main tree structure for lottery analysis. A `LoTree` is immutable once
`Build` returns, which allows a single tree to be cached per date window and
shared across concurrent requests.

**Fields:**
- `Anchor *LoNode`: Root node of the tree
- `Lottery int`: Size of the lottery (e.g., 37 for 6/37)
- `BuildCalls int`: Number of forms built into tree

**Key Methods:**
- `NewLoTree(lottery)`: Create a new tree
- `Build(data, howMany)`: Build tree from historical data; returns receiver for chaining
- `CollectNodes(size, mode)`: Single read-only traversal returning every group of exactly `size` numbers with its occurrence count, sorted by count (strong = most-frequent first, weak = least-frequent first). Stable sort preserves ascending-number order for equal counts.
- `AnalyzeArray(form, maxSize)`: Traverse the tree following only the user's form numbers, producing `AnalyzeResponse` frequency groups in one forward pass.

**NodeEntry** (returned by `CollectNodes`):
- `Numbers []int`: The number group
- `Count int`: How many archived draws contain this group

### LotteryArchive (`lottery_archive.go`)

An immutable, cached owner of historical draw data for a specific date window.
It owns the regular-number prefix tree, the strong-number prefix tree, a
winning-combination hash set, and the raw draw metadata. All query methods are
read-only and safe for concurrent use.

**Fields:**
- `Lottery int`: Lottery type constant
- `From`, `To time.Time`: Date window bounds
- `Draws []models.LotteryResult`: Raw draw metadata
- `Tree *LoTree`: Regular-number prefix tree (depth 6)
- `StrongTree *LoTree`: Strong-number prefix tree
- `WinningCombos map[uint64]bool`: Hash set of past winning combinations

**Key Methods:**
- `NewLotteryArchive(lottery, from, to, draws)`: Construct and build both trees
- `Size()`: Number of draws in the archive
- `MaxNumber()`: Maximum regular number for this lottery type (37 for Lotto)
- `RankNumbers(mode)`: Return all regular numbers sorted by frequency (thin wrapper over `CollectNodes(1, mode)`)
- `TopStrong(mode)`: Return the most (strong) or least (weak) frequent strong number
- `TopGroups(howMany, size, mode)`: Return the top `howMany` groups of `size` numbers by frequency
- `HasWon(combo)`: O(1) hash-set check whether a combination has already won
- `AnalyzeForm(form)`: Produce `AnalyzeResponse` with frequency groups for all subsets of `form` up to size 6
- `NewFormGenerator(formType, mode)`: Create a request-scoped `FormGenerator` borrowing this archive

### FormGenerator (`form_generator.go`)

Request-scoped mutable state for form generation. Borrows a `LotteryArchive`
read-only; all mutable state (`tries`, `seen`, `results`) belongs to one
request, preventing leakage between concurrent RPCs.

**Key Methods:**
- `Generate(howMany, willBe)`: Return up to `howMany` distinct non-winning forms, each with `willBe` numbers front-loaded and the top strong number appended

## Usage Examples

### Basic Tree Building

```go
package main

import (
    "github.com/lihai1/stat-tree-server/internal/lotterytree"
)

func main() {
    // Create a new tree for 6/37 lottery
    tree := lotterytree.NewLoTree(37)

    // Build tree from historical data
    data := [][]int{
        {1, 2, 3, 4, 5, 6},
        {7, 8, 9, 10, 11, 12},
        {13, 14, 15, 16, 17, 18},
    }
    tree.Build(data, 6)

    // Collect single-number frequencies (strong = most frequent first)
    entries := tree.CollectNodes(1, string(lotterytree.Strong))
    // entries[0].Numbers == []int{1}, entries[0].Count == 1

    // Collect pair frequencies
    pairs := tree.CollectNodes(2, string(lotterytree.Strong))
}
```

### Archive and Form Generation

```go
package main

import (
    "time"
    "github.com/lihai1/stat-tree-server/internal/lotterytree"
    "github.com/lihai1/stat-tree-server/internal/models"
)

func main() {
    draws := []models.LotteryResult{
        {DrawNumber: 1, DrawDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
            Numbers: []int{1, 2, 3, 4, 5, 6}, Strong: 5},
        {DrawNumber: 2, DrawDate: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
            Numbers: []int{7, 8, 9, 10, 11, 12}, Strong: 3},
    }

    // Build an immutable archive (trees + winning-combo hash set)
    arch := lotterytree.NewLotteryArchive(
        lotterytree.Lottery, time.Time{}, time.Time{}, draws,
    )

    // Rank numbers by frequency
    ranked := arch.RankNumbers(string(lotterytree.Strong))

    // Check if a combination has already won
    won := arch.HasWon([]int{1, 2, 3, 4, 5, 6}) // true

    // Generate non-winning forms
    gen := arch.NewFormGenerator(6, string(lotterytree.Strong))
    forms := gen.Generate(3, []int{1, 2}) // 3 forms, each containing 1 and 2
}
```

## Integration with Services

The package is used by the service layer through `LotteryManager`, which caches
`LotteryArchive` instances per date window in an LRU cache:

```go
type LotteryService struct {
    lotteryv1.UnimplementedLotteryServiceServer
    manager *LotteryManager
}

func NewLotteryService(manager *LotteryManager) *LotteryService {
    return &LotteryService{manager: manager}
}

// Each RPC resolves a cached archive for the request's date window:
func (s *LotteryService) GenerateForm(ctx context.Context, req *lotteryv1.GenerateFormRequest) (*lotteryv1.GenerateFormResponse, error) {
    arch, err := s.archive(ctx, req.GetDateWindow())
    if err != nil {
        return nil, err
    }
    gen := arch.NewFormGenerator(int(req.GetFormType()), req.GetStrength().String())
    forms := gen.Generate(int(req.GetHowMany()), intSlice(req.GetWillBe()))
    // ...
}
```

## Algorithms

### Tree Building

The tree is built recursively from historical lottery forms. Each node represents a number in the sequence, with child nodes representing subsequent numbers. The `Count` field tracks how many times a particular sequence has occurred. A `LoTree` is immutable once `Build` returns.

### Group Collection (CollectNodes)

`CollectNodes` replaces the former `BestPares` algorithm. Instead of re-traversing the tree once per requested group and mutating tree state to deduplicate, it walks the tree once in a single read-only pass, collects every group of the requested size with its count, and sorts the result stably by count. This is used for single-number frequencies (`size=1`), pairs (`size=2`), and larger groups up to `maxGroupSize` (6).

### Form Generation

`FormGenerator.Generate` uses `RankNumbers` (which delegates to `CollectNodes(1, mode)`) to order the candidate pool by frequency, front-loads any `willBe` numbers, then uses recursive backtracking to produce non-winning combinations. The winning check uses `HasWon`, an O(1) hash-set lookup on the archive's `WinningCombos` map, replacing the former O(m) `InTree` traversal.

### Form Analysis

The `AnalyzeForm` method traverses the tree with a user's form, recording the frequency of each number sequence found in the historical data. All numbers in the form are treated as regular numbers — no trailing number is stripped as a strong number.

## Historical Format & Archive Default

The Israeli Lotto has undergone format changes that affect what the "strong
number" means and which regular-number range is valid. The archive default
start date (`defaultArchiveFrom = 2004-02-12` in `lottery_manager.go`) is
chosen to exclude the older format.

### Format eras

| Period | Regular numbers | Strong number | Notes |
|--------|----------------|---------------|-------|
| Pre-2004 | 1–49 (6/49) | None — field 9 is a 7th regular | Old format. Not comparable to the current game. |
| 2004-02-12 → 2011-03-01 | 1–37 | 1–7 (mostly) | Current format. A handful of CSV rows show strong 8–10 (data errors or transitional artifacts). |
| Post-2011-03-01 | 1–37 | 1–7 (strictly) | Clean, current format. |

### Why the default is 2004-02-12

Draws before this date used the 6/49 format: regular numbers ranged up to 49
and there was no separate strong number in the 1–7 range. Mixing those draws
into the modern archive would corrupt frequency statistics, generate forms
with numbers outside 1–37, and produce strong-number rankings from data that
is not a strong number at all.

The `LotteryManager` applies this default when the request's `from` bound is
zero (unset). A caller may explicitly request an earlier window — the service
accepts it and builds the archive from whatever draws exist — but the results
will reflect the old format and should be interpreted accordingly.

### Strong tree sizing

The strong tree is built with `NewLoTree(maxStrong+2)`, where `maxStrong = 7`.
This creates a `Next` slice of length 9 (indices 0–8), so strong numbers 1–9
are stored. Values outside this range (e.g. 10–49 from pre-2004 rows) are
silently dropped by the bounds check in `LoNode.Build` (`treeIndex < len(n.Next)`).

`TopStrong` only iterates indices 0–6 (strong numbers 1–7), so the extra
slots for 8 and 9 are stored but never selected. They exist as a small
buffer for the transitional 2004–2011 period and do not affect any output.

## Testing

The package includes comprehensive Ginkgo tests:

```bash
# Run tests
make test

# Or directly with ginkgo
~/go/bin/ginkgo -v --cover ./internal/lottery-tree/...
```

Test coverage: ~25% (can be improved with additional test cases)

## Performance Considerations

- Tree building is O(n*m) where n is number of forms and m is form length
- Tree queries are O(m) where m is the sequence length
- Memory usage grows with the number of unique sequences in historical data
- Consider using pagination or batching for large datasets

## Future Enhancements

- Add more sophisticated generation strategies
- Improve test coverage
- Add benchmarking tests
- Add support for additional lottery types

## Origin

This package was refactored from Java code located in `statistiloto-server/src/main/java/com/lotteryTools`. The original Java classes were:
- `LoNode.java` → `node.go`
- `LoTree.java` → `tree.go`
- `LotteryList.java` → `list.go` (deleted — dead code)
- `LotteryArray.java` → `lottery_array.go` (deleted — replaced by `lottery_archive.go` + `form_generator.go`)
- `lotteryToolsCommon.java` → `types.go` (Common)
- `lotteryToolsTypes.java` → `types.go` (LotteryType)

## License

This package is part of the Statistiloto project.
