package lotterytree

import "sort"

// LoTree is a prefix tree over sorted lottery draws. Each node at depth K
// represents one K-number group, and its Count is how many archived draws
// contain that group. Building it once therefore materialises every group
// frequency the service needs: depth 1 gives single-number frequencies,
// depth 2 pairs, depth 3 triples, and so on.
//
// A LoTree is immutable once Build returns, which is what allows a single
// tree to be cached per date window and shared across concurrent requests.
type LoTree struct {
	Anchor     *LoNode
	Size       int
	Lottery    int
	BuildCalls int
}

// NewLoTree creates a new lottery tree sized for the given number range.
func NewLoTree(lottery int) *LoTree {
	return &LoTree{
		Lottery: lottery,
		Anchor:  NewLoNode(0, lottery),
	}
}

// Build inserts every draw in data into the tree, recording group counts up
// to howMany levels deep. It returns the receiver so callers can chain.
func (t *LoTree) Build(data [][]int, howMany int) *LoTree {
	for i := 0; i < len(data); i++ {
		t.Anchor.Build(data[i], 0, howMany)
	}
	t.BuildCalls = len(data)
	return t
}

// NodeEntry is one number group and how many archived draws contain it.
type NodeEntry struct {
	Numbers []int
	Count   int
}

// CollectNodes walks the tree once and returns every group of exactly the
// given size together with its occurrence count, ordered by count. Mode
// "weak" orders least-frequent first; any other mode orders most-frequent
// first. Ordering is stable, so groups with equal counts keep their natural
// ascending-number order.
//
// This replaces the former BestPares, which re-traversed the whole tree once
// per requested group and mutated tree state to deduplicate. A single pass
// plus a sort yields the same information without touching the tree.
func (t *LoTree) CollectNodes(size int, mode string) []NodeEntry {
	if size <= 0 || t.Anchor == nil {
		return nil
	}

	var entries []NodeEntry
	for _, child := range t.Anchor.Next {
		if child != nil {
			child.collectNodes(size-1, nil, &entries)
		}
	}

	sort.SliceStable(entries, func(a, b int) bool {
		if mode == string(Weak) {
			return entries[a].Count < entries[b].Count
		}
		return entries[a].Count > entries[b].Count
	})
	return entries
}

// collectNodes appends this node's group when the remaining depth is
// exhausted, otherwise recurses into its children. prefix holds the numbers
// chosen above this node and is copied per entry, so no caller shares state.
func (n *LoNode) collectNodes(remaining int, prefix []int, out *[]NodeEntry) {
	numbers := make([]int, len(prefix)+1)
	copy(numbers, prefix)
	numbers[len(prefix)] = n.Num

	if remaining == 0 {
		*out = append(*out, NodeEntry{Numbers: numbers, Count: n.Count})
		return
	}
	for _, child := range n.Next {
		if child != nil {
			child.collectNodes(remaining-1, numbers, out)
		}
	}
}
