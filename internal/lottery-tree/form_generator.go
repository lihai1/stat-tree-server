package lotterytree

import "sort"

// minFormSize is the smallest playable form: the six regular numbers.
const minFormSize = 6

// FormGenerator produces lottery forms that have never won before.
//
// It holds everything derived from a single request — the frequency-ordered
// candidate list, the partial results, and the backtracking cursor — while
// borrowing the archive by reference and never mutating it. One generator
// belongs to one request, so concurrent requests against the same cached
// archive stay independent.
type FormGenerator struct {
	archive  *LotteryArchive
	formType int
	mode     string

	// tries is the working candidate list. The search permutes it in place,
	// so it is reset from the frequency ranking before each form.
	tries  []int
	strong int

	results [][]int
	seen    map[uint64]bool

	// skip is how many leading entries the search must keep fixed, used to
	// pin the caller's requested numbers (willBe) at the front.
	skip int

	// caseL alternates the recursion's branching, preserving the original
	// algorithm's traversal order.
	caseL int
}

// Generate returns up to howMany distinct forms, each of formType numbers
// with the chosen strong number appended. Numbers listed in willBe appear in
// every form. Forms that have already won are rejected.
func (g *FormGenerator) Generate(howMany int, willBe []int) [][]int {
	if howMany <= 0 {
		return nil
	}

	// Frequency ranking and the strong number depend only on the archive,
	// so they are read once from the cached trees rather than per form.
	ranked := g.archive.RankNumbers(g.mode)
	g.strong = g.archive.TopStrong(g.mode)

	if len(ranked) < g.formType {
		return nil
	}

	g.tries = make([]int, len(ranked))
	g.results = make([][]int, 0, howMany)
	g.seen = make(map[uint64]bool, howMany)

	for i := 0; i < howMany; i++ {
		// The search leaves tries permuted when it succeeds, so restore the
		// frequency ranking before looking for the next form.
		copy(g.tries, ranked)
		g.regroup(willBe)
		g.search(g.skip, g.formType)
		if !g.addResult() {
			// No further non-winning combination is reachable.
			break
		}
	}
	return g.results
}

// regroup moves the caller's requested numbers to the front of tries and
// records how many leading positions the search must leave alone.
func (g *FormGenerator) regroup(willBe []int) {
	g.skip = 0
	if len(willBe) == 0 {
		return
	}

	start := 0
	for _, want := range willBe {
		for j := start; j < len(g.tries); j++ {
			if g.tries[j] == want {
				g.tries[start], g.tries[j] = g.tries[j], g.tries[start]
				start++
				break
			}
		}
	}
	g.skip = start
}

// search permutes tries looking for a leading formType numbers that form a
// combination which is both unseen in this batch and absent from history.
// It returns true as soon as one is found, leaving tries in that state.
func (g *FormGenerator) search(start, check int) bool {
	g.caseL = (g.caseL + 1) % 2

	if g.notInResults() && g.allFormsCheck() {
		return true
	}
	if start == g.formType || check == len(g.tries) {
		return false
	}

	g.tries[start], g.tries[check] = g.tries[check], g.tries[start]
	if g.search(start+(g.caseL+1)%2, check+g.caseL) {
		return true
	}
	if g.search(start+g.caseL, check+(g.caseL+1)%2) {
		return true
	}
	g.tries[start], g.tries[check] = g.tries[check], g.tries[start]
	return false
}

// notInResults reports whether the current leading combination has not
// already been emitted in this batch. The strong number is excluded from
// the identity, so two forms differing only by strong count as duplicates.
func (g *FormGenerator) notInResults() bool {
	return !g.seen[ComboMask(g.leading())]
}

// allFormsCheck reports whether none of the six-number combinations playable
// from the current leading numbers has won before. A plain form yields one
// combination; a systematic form of N numbers yields C(N,6).
func (g *FormGenerator) allFormsCheck() bool {
	for _, combo := range GenerateCombinations(g.leading(), minFormSize) {
		if g.archive.HasWon(combo) {
			return false
		}
	}
	return true
}

// leading returns the current candidate form: the first formType entries of
// tries.
func (g *FormGenerator) leading() []int {
	size := g.formType
	if size > len(g.tries) {
		size = len(g.tries)
	}
	return g.tries[:size]
}

// addResult records the current candidate as a result, appending the strong
// number, and marks it seen so later iterations skip it. It reports whether
// a result was added.
func (g *FormGenerator) addResult() bool {
	leading := g.leading()
	if len(leading) == 0 {
		return false
	}

	combo := make([]int, len(leading))
	copy(combo, leading)
	sort.Ints(combo)

	mask := ComboMask(combo)
	if g.seen[mask] {
		return false
	}
	g.seen[mask] = true

	form := make([]int, len(combo)+1)
	copy(form, combo)
	form[len(combo)] = g.strong
	g.results = append(g.results, form)
	return true
}
