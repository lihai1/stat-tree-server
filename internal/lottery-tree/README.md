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
├── types.go          # Type definitions (Common, LotteryType)
├── node.go           # LoNode - tree node implementation
├── tree.go           # LoTree - tree structure and algorithms
├── list.go           # LotteryList - linked list for form management
├── lottery_array.go  # LotteryArray - main analysis structure
└── README.md         # This file
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
- `Build(form, current, howMany)`: Build tree structure from a form
- `InTree(data, idata, current, level)`: Check if sequence exists in tree

### LoTree (`tree.go`)

The main tree structure for lottery analysis.

**Fields:**
- `Anchor *LoNode`: Root node of the tree
- `Lottery int`: Size of the lottery (e.g., 37 for 6/37)
- `Pares [][]int`: Stored pairs for analysis
- `BuildCalls int`: Number of forms built into tree

**Key Methods:**
- `NewLoTree(lottery)`: Create a new tree
- `Build(data, howMany)`: Build tree from historical data
- `InTree(data)`: Check if data exists in tree
- `NodesInTree()`: Count total nodes
- `NodesInLevel(level)`: Count nodes at specific level
- `BestPares(sumOfPares, howMany, weakStrong)`: Find best pairs based on strength

### LotteryList (`list.go`)

A linked list implementation for storing lottery forms.

**Fields:**
- `Anchor *LoNode`: Head of the list
- `Node *LoNode`: Current position in list
- `Size int`: Number of forms in list

**Key Methods:**
- `NewLotteryList()`: Create empty list
- `Add(form)`: Add form (prevents duplicates)
- `ToHeadList()`: Reset to head
- `NextForm()`: Get next form
- `HasMoreForms()`: Check if more forms available
- `Pieces()`: Get all forms as 2D array

### LotteryArray (`lottery_array.go`)

The main analysis structure that coordinates all components.

**Fields:**
- `FileName string`: Data file name
- `Lottery LotteryType`: Type of lottery
- `Archive [][]int`: Historical lottery results
- `StrongArchive [][]int`: Strong number history
- `Query *LoTree`: Analysis tree
- `TriesType string`: Generation strategy

**Key Methods:**
- `NewLotteryArray(fileName, lottery)`: Create new instance
- `Sort(arr)`: Sort array ascending
- `CutArrayTo(arr, size)`: Trim array to size
- `HowManyNumbersInLeft(test, gotNums, searchLength)`: Count matches
- `WonBig(check, strong)`: Check if form won
- `BuildTree(howMany)`: Build analysis tree
- `RepeatingPares(paresGeneral, howMany, weakStrong)`: Find repeating pairs
- `AnalyzeForm(form)`: Analyze a form against tree
- `Tries()`: Get number array for generation
- `TooManyFollows(lottery)`: Check for too many consecutive numbers

## Usage Examples

### Basic Tree Building

```go
package main

import (
    "statistiloto-v2/stat-tree-server/internal/lotterytree"
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
    
    // Check if a sequence exists
    exists := tree.InTree([]int{1, 2, 3, 4, 5, 6})
    
    // Count nodes
    totalNodes := tree.NodesInTree()
    levelNodes := tree.NodesInLevel(2)
}
```

### Lottery Array Analysis

```go
package main

import (
    "statistiloto-v2/stat-tree-server/internal/lotterytree"
)

func main() {
    // Create lottery array
    la := lotterytree.NewLotteryArray("data.txt", lotterytree.Lottery)
    
    // Load historical data (you would implement file loading)
    // la.Archive = loadHistoricalData()
    
    // Build analysis tree
    la.BuildTree(6)
    
    // Find frequent pairs
    pairs := la.RepeatingPares(10, 2, string(lotterytree.Strong))
    
    // Analyze a user's form
    userForm := []int{1, 5, 10, 15, 20, 30}
    analysis := la.AnalyzeForm(userForm)
    
    // Generate numbers
    tries := la.Tries()
    sorted := la.Sort(tries)
    selected := la.CutArrayTo(sorted, 6)
}
```

### Using Lottery List

```go
package main

import (
    "statistiloto-v2/stat-tree-server/internal/lotterytree"
)

func main() {
    // Create list
    list := lotterytree.NewLotteryList()
    
    // Add forms
    list.Add([]int{1, 2, 3, 4, 5, 6})
    list.Add([]int{7, 8, 9, 10, 11, 12})
    
    // Try adding duplicate (won't be added)
    list.Add([]int{1, 2, 3, 4, 5, 6})
    
    // Iterate through forms
    list.ToHeadList()
    for list.HasMoreForms() {
        form := list.NextForm()
        // Process form
    }
    
    // Get all forms
    allForms := list.Pieces()
}
```

## Integration with Services

The package is designed to be used by service layers, such as the `LotteryService` in the application:

```go
type LotteryService struct {
    lotteryv1.UnimplementedLotteryServiceServer
    lotteryArray *lotterytree.LotteryArray
}

func NewLotteryService() *LotteryService {
    return &LotteryService{
        lotteryArray: lotterytree.NewLotteryArray("", lotterytree.Lottery),
    }
}
```

## Algorithms

### Tree Building

The tree is built recursively from historical lottery forms. Each node represents a number in the sequence, with child nodes representing subsequent numbers. The `Count` field tracks how many times a particular sequence has occurred.

### Pair Analysis

The `BestPares` algorithm traverses the tree to find the most (or least) frequent number pairs based on the `weakStrong` parameter. This is useful for identifying hot and cold numbers.

### Form Analysis

The `AnalyzeForm` method traverses the tree with a user's form, recording the frequency of each number sequence found in the historical data. This provides insights into how "hot" or "cold" a particular combination is.

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

- Implement file loading for historical data
- Add more sophisticated generation strategies
- Improve test coverage
- Add benchmarking tests
- Implement caching for frequently accessed data
- Add support for additional lottery types

## Origin

This package was refactored from Java code located in `statistiloto-server/src/main/java/com/lotteryTools`. The original Java classes were:
- `LoNode.java` → `node.go`
- `LoTree.java` → `tree.go`
- `LotteryList.java` → `list.go`
- `LotteryArray.java` → `lottery_array.go`
- `lotteryToolsCommon.java` → `types.go` (Common)
- `lotteryToolsTypes.java` → `types.go` (LotteryType)

## License

This package is part of the Statistiloto project.
