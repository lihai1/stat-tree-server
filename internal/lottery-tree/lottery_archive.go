package lotterytree

import (
	"math"
	"sort"
	"time"

	"github.com/lihai1/stat-tree-server/internal/models"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

// maxGroupSize is the deepest group the archive can answer for. The game
// draws six regular numbers, so groups larger than six have no meaning and
// the trees are built to this depth.
const maxGroupSize = 6

// LotteryArchive is a preloaded, immutable view of the historical draws in
// one date window, together with every structure derived from them:
//
//   - Tree        prefix tree over the regular numbers, depth maxGroupSize.
//     Node counts at depth K are the frequencies of every
//     K-number group, so single numbers, pairs and triples
//     are all read from this one structure.
//   - StrongTree  prefix tree over the strong numbers, depth 1.
//   - WinningCombos  bitmask set of every six-number combination that has
//     already won, for O(1) "has this form won before" checks.
//
// Nothing on an archive is mutated after NewLotteryArchive returns, so a
// single instance is safely shared by concurrent requests. Per-request
// generation state lives in FormGenerator, obtained via NewFormGenerator.
type LotteryArchive struct {
	Lottery LotteryType
	From    time.Time
	To      time.Time

	// Draws keeps the full rows (draw number, date, prize amounts) that
	// callers such as Simulate need beyond the bare numbers.
	Draws []models.LotteryResult

	Numbers [][]int // the regular numbers of each draw
	Strongs [][]int // the strong number of each draw, as a one-element row

	WinningCombos map[uint64]bool
	Tree          *LoTree
	StrongTree    *LoTree
}

// NewLotteryArchive builds every derived structure for the given draws once.
// draws may be empty, in which case the archive answers queries with zero
// values rather than failing.
func NewLotteryArchive(lottery LotteryType, from, to time.Time, draws []models.LotteryResult) *LotteryArchive {
	a := &LotteryArchive{
		Lottery: lottery,
		From:    from,
		To:      to,
		Draws:   draws,
		Numbers: make([][]int, 0, len(draws)),
		Strongs: make([][]int, 0, len(draws)),
	}

	a.WinningCombos = make(map[uint64]bool, len(draws))
	for _, d := range draws {
		a.Numbers = append(a.Numbers, d.Numbers)
		a.Strongs = append(a.Strongs, []int{d.Strong})

		// Only complete six-number draws define a winning combination.
		if len(d.Numbers) >= 6 {
			combo := make([]int, 6)
			copy(combo, d.Numbers[:6])
			sort.Ints(combo)
			a.WinningCombos[ComboMask(combo)] = true
		}
	}

	a.Tree = NewLoTree(a.MaxNumber()).Build(a.Numbers, maxGroupSize)
	a.StrongTree = NewLoTree(maxStrong+2).Build(a.Strongs, 1)

	return a
}

// maxStrong is the highest valid strong number in the current game. The
// strong tree is sized slightly larger because the archive also contains
// historical draws whose strong number exceeded this range.
const maxStrong = 7

// MaxNumber returns the highest regular number this lottery variant draws.
func (a *LotteryArchive) MaxNumber() int {
	switch a.Lottery {
	case TripleSeven:
		return 70
	case OneTwoThree:
		return 10
	default:
		return 37
	}
}

// Size returns how many draws the archive covers.
func (a *LotteryArchive) Size() int {
	return len(a.Draws)
}

// RankNumbers orders every regular number by how often it has been drawn,
// most frequent first ("strong") or least frequent first ("weak").
//
// This is a thin convenience wrapper over CollectNodes(1, mode): the
// depth-1 tree nodes already hold the per-number counts, so CollectNodes
// does the walk and sort once and RankNumbers just flattens the
// []NodeEntry into the []int the form generator needs. When the archive
// is empty (no draws), every valid number 1..MaxNumber is returned in
// ascending order so generation can still proceed against a cold or test
// database.
func (a *LotteryArchive) RankNumbers(mode string) []int {
	max := a.MaxNumber()

	if a.Tree == nil || a.Tree.Anchor == nil || a.Size() == 0 {
		ranked := make([]int, max)
		for i := 0; i < max; i++ {
			ranked[i] = i + 1
		}
		return ranked
	}

	entries := a.Tree.CollectNodes(1, mode)
	ranked := make([]int, len(entries))
	for i, e := range entries {
		ranked[i] = e.Numbers[0]
	}
	return ranked
}

// TopStrong returns the most frequent strong number ("strong" mode) or the
// least frequent one ("weak" mode), considering only the 1..maxStrong range
// that the current game uses. Counts come from the strong tree's depth-1
// nodes. Returns 0 when the archive holds no valid strong numbers.
func (a *LotteryArchive) TopStrong(mode string) int {
	if a.Lottery != Lottery || a.StrongTree == nil || a.StrongTree.Anchor == nil {
		return 0
	}

	best, bestCount := 0, -1
	if mode == string(Weak) {
		bestCount = math.MaxInt32
	}

	for i := 0; i < maxStrong && i < len(a.StrongTree.Anchor.Next); i++ {
		node := a.StrongTree.Anchor.Next[i]
		if node == nil {
			continue
		}
		num, count := i+1, node.Count
		if mode == string(Weak) {
			if count < bestCount {
				best, bestCount = num, count
			}
			continue
		}
		if count > bestCount {
			best, bestCount = num, count
		}
	}
	return best
}

// TopGroups returns the howMany most frequent number groups of the given
// size ("strong" mode) or the least frequent ones ("weak" mode). Size must
// be between 1 and maxGroupSize.
func (a *LotteryArchive) TopGroups(howMany, size int, mode string) []NodeEntry {
	if a.Tree == nil || size <= 0 || size > maxGroupSize || howMany <= 0 {
		return nil
	}
	entries := a.Tree.CollectNodes(size, mode)
	if len(entries) > howMany {
		entries = entries[:howMany]
	}
	return entries
}

// AnalyzeForm reports how often each subset of the given form, up to
// maxGroupSize numbers, appears in the archive. Every number in form is
// treated as a regular number; the traversal explores all subsets, so forms
// of any length are handled without truncation.
func (a *LotteryArchive) AnalyzeForm(form []int) *lotteryv1.AnalyzeResponse {
	if a.Tree == nil {
		return emptyAnalyzeResponse()
	}
	return a.Tree.AnalyzeArray(form, maxGroupSize)
}

// HasWon reports whether the given combination has already won. The
// combination is matched as a set, so element order does not matter.
func (a *LotteryArchive) HasWon(combo []int) bool {
	if len(a.WinningCombos) == 0 {
		return false
	}
	return a.WinningCombos[ComboMask(combo)]
}

// NewFormGenerator returns a generator that produces forms from this
// archive. The generator owns all mutable state, so several may run
// concurrently against the same archive.
func (a *LotteryArchive) NewFormGenerator(formType int, mode string) *FormGenerator {
	if formType < minFormSize {
		formType = minFormSize
	}
	return &FormGenerator{
		archive:  a,
		formType: formType,
		mode:     mode,
	}
}

// ComboMask returns a bitmask identifying a set of numbers: bit N is set
// when number N is present. For the 1-37 range this fits in a single uint64,
// so two combinations are equal exactly when their masks are equal, and the
// mask is independent of element order.
func ComboMask(nums []int) uint64 {
	var mask uint64
	for _, n := range nums {
		if n > 0 && n < 64 {
			mask |= 1 << uint64(n)
		}
	}
	return mask
}

// GenerateCombinations returns every k-sized combination of numbers, each as
// a slice in the same relative order as the input.
func GenerateCombinations(numbers []int, k int) [][]int {
	n := len(numbers)
	if k > n || k <= 0 {
		return nil
	}
	if k == n {
		combo := make([]int, n)
		copy(combo, numbers)
		return [][]int{combo}
	}

	result := make([][]int, 0, binomialCount(n, k))
	indices := make([]int, k)
	for i := range indices {
		indices[i] = i
	}

	for {
		combo := make([]int, k)
		for i, idx := range indices {
			combo[i] = numbers[idx]
		}
		result = append(result, combo)

		// Advance to the next combination in lexicographic index order.
		i := k - 1
		for i >= 0 && indices[i] == n-k+i {
			i--
		}
		if i < 0 {
			break
		}
		indices[i]++
		for j := i + 1; j < k; j++ {
			indices[j] = indices[j-1] + 1
		}
	}
	return result
}

// binomialCount returns C(n, k), or 0 when k falls outside 0..n. It is used
// both to presize combination slices and by the simulation to count how many
// combinations fall into each prize tier without enumerating them.
func binomialCount(n, k int) int {
	if k < 0 || k > n || n < 0 {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n-k {
		k = n - k
	}
	result := 1
	for i := 0; i < k; i++ {
		result = result * (n - i) / (i + 1)
	}
	return result
}

// Binomial returns C(n, k), or 0 when k falls outside 0..n.
func Binomial(n, k int) int {
	return binomialCount(n, k)
}

// AnalyzeArray traverses the tree for the given form and builds the
// AnalyzeResponse in a single forward pass. FrequencyEntry messages are
// appended directly to the correct FrequencyGroup during recursion — the
// prefix is passed by value, so each entry is constructed once with no
// mutable accumulator state.
func (t *LoTree) AnalyzeArray(form []int, maxSize int) *lotteryv1.AnalyzeResponse {
	groups := make([]*lotteryv1.FrequencyGroup, maxGroupSize)
	for i := range groups {
		groups[i] = &lotteryv1.FrequencyGroup{Size: int32(i + 1)}
	}

	t.Anchor.analyzeBuild(form, 0, 0, maxSize, groups, nil)

	for _, g := range groups {
		sort.SliceStable(g.Entries, func(a, b int) bool {
			return g.Entries[a].GetCount() > g.Entries[b].GetCount()
		})
	}

	return &lotteryv1.AnalyzeResponse{
		FrequencyGroups: groups,
		ArchiveSize:     int32(t.BuildCalls),
	}
}

func emptyAnalyzeResponse() *lotteryv1.AnalyzeResponse {
	groups := make([]*lotteryv1.FrequencyGroup, maxGroupSize)
	for i := range groups {
		groups[i] = &lotteryv1.FrequencyGroup{Size: int32(i + 1)}
	}
	return &lotteryv1.AnalyzeResponse{FrequencyGroups: groups}
}

func (n *LoNode) analyzeBuild(form []int, current, size, maxSize int,
	groups []*lotteryv1.FrequencyGroup, prefix []int32) {
	if size == maxSize {
		return
	}
	for i := current; i < len(form); i++ {
		treeIndex := form[i] - n.Num - 1
		if treeIndex >= 0 && treeIndex < len(n.Next) && n.Next[treeIndex] != nil {
			child := n.Next[treeIndex]
			newSize := size + 1

			// Build this entry's numbers from prefix + current number.
			numbers := make([]int32, newSize)
			copy(numbers, prefix)
			numbers[size] = int32(form[i])

			groups[newSize-1].Entries = append(groups[newSize-1].Entries,
				&lotteryv1.FrequencyEntry{
					Numbers: numbers,
					Count:   int32(child.Count),
				})

			child.analyzeBuild(form, i+1, newSize, maxSize, groups, numbers)
		}
	}
}
