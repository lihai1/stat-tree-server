package lotterytree

import (
	"math/rand"
	"sort"
	"time"
)

// LotteryArray represents the main lottery analysis structure
type LotteryArray struct {
	FileName      string
	Lottery       LotteryType
	Data          [][]string
	Archive       [][]int
	StrongArchive [][]int
	Results       [][]int
	Strongs       []int
	Query         *LoTree
	MinNum        int
	Consistancy   int
	Skip          int
	SetRandom     bool
	TestCount     int
	Pieces        *LotteryList
	WinsInForm    int
	WinsInFormC   int
	TriesType     string
	FormType      int
	ArchiveStart  time.Time
	ArchiveEnd    time.Time
	tries         []int
	caseL         int
}

// NewLotteryArray creates a new lottery array
func NewLotteryArray(fileName string, lottery LotteryType) *LotteryArray {
	return &LotteryArray{
		FileName:    fileName,
		Lottery:     lottery,
		MinNum:      6,
		Consistancy: 3,
		SetRandom:   false,
		TriesType:   "frequent",
	}
}

// Sort sorts an integer slice in ascending order
func (la *LotteryArray) Sort(arr []int) []int {
	if arr == nil {
		return nil
	}
	sorted := make([]int, len(arr))
	copy(sorted, arr)
	sort.Ints(sorted)
	return sorted
}

// NewPointerSort creates a sorted copy of the array
func (la *LotteryArray) NewPointerSort(arr []int) []int {
	return la.Sort(arr)
}

// CutArrayTo returns a sub-array of the specified size
func (la *LotteryArray) CutArrayTo(arr []int, size int) []int {
	if size > len(arr) {
		size = len(arr)
	}
	nums := make([]int, size)
	copy(nums, arr[:size])
	return nums
}

// HowManyNumbersInLeft counts how many numbers from gotNums appear in test
func (la *LotteryArray) HowManyNumbersInLeft(test, gotNums []int, searchLength int) int {
	count := 0
	if searchLength > len(test) {
		searchLength = len(test)
	}
	for i := 0; i < searchLength; i++ {
		for j := 0; j < len(gotNums); j++ {
			if test[i] == gotNums[j] {
				count++
			}
		}
	}
	return count
}

// WonBig checks if a form has won a big prize
func (la *LotteryArray) WonBig(check []int, strong int) int {
	switch la.Lottery {
	case Lottery:
		for i := 0; i < len(la.Archive); i++ {
			if la.equalsSix(la.Archive[i], check) {
				if len(la.StrongArchive) > i && la.StrongArchive[i][0] == strong {
					return 1
				}
				return 2
			}
		}
	case TripleSeven:
		for i := 0; i < len(la.Archive); i++ {
			if la.HowManyNumbersInLeft(la.Archive[i], check, 6) == 7 {
				return 1
			}
		}
	case OneTwoThree:
		for i := 0; i < len(la.Archive); i++ {
			if la.equalsThree(la.Archive[i], check) {
				return 1
			}
		}
	}
	return 0
}

func (la *LotteryArray) equalsSix(a, b []int) bool {
	if len(a) < 6 || len(b) < 6 {
		return false
	}
	for i := 0; i < 6; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (la *LotteryArray) equalsThree(a, b []int) bool {
	if len(a) < 3 || len(b) < 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BuildTree builds the lottery tree
func (la *LotteryArray) BuildTree(howMany int) {
	switch la.Lottery {
	case Lottery:
		la.Query = NewLoTree(37)
	case TripleSeven:
		la.Query = NewLoTree(70)
	case OneTwoThree:
		la.Query = NewLoTree(10)
	}
	la.Query.Build(la.Archive, howMany)
}

// RepeatingPares finds repeating pairs based on strength
func (la *LotteryArray) RepeatingPares(paresGeneral, howMany int, weakStrong string) [][]int {
	ans := la.Query.BestPares(paresGeneral, howMany, weakStrong)
	return ans
}

// AnalyzeForm analyzes a form against the tree
func (la *LotteryArray) AnalyzeForm(form []int) [][]int {
	max := 6
	if la.Lottery == Lottery {
		max = 6
	}
	la.BuildTree(max)
	return la.Query.AnalyzeArray(form, max)
}

// SetFrequentCombo sets the tries type to frequent
func (la *LotteryArray) SetFrequentCombo() {
	la.TriesType = "frequent"
}

// SetLessFrequentCombo sets the tries type to less frequent
func (la *LotteryArray) SetLessFrequentCombo() {
	la.TriesType = "less frequent"
}

// SetRandomCombo sets the tries type to random
func (la *LotteryArray) SetRandomCombo() {
	la.TriesType = "random"
}

// Consistance sets the consistancy based on form type
func (la *LotteryArray) Consistance(formType int) {
	switch formType {
	case 12:
		la.Consistancy = 6
	case 11:
		la.Consistancy = 5
	case 10, 9, 8:
		la.Consistancy = 4
	case 7:
		la.Consistancy = 4
	}
}

// Swap swaps elements between two arrays
func (la *LotteryArray) Swap(arrX []int, x int, arrY []int, y int) {
	tmp := arrX[x]
	arrX[x] = arrY[y]
	arrY[y] = tmp
}

// ReGroup reorganizes the tries array based on willBe
func (la *LotteryArray) ReGroup(willBe []int) {
	if len(willBe) > 0 {
		start := 0
		for i := 0; i < len(willBe); i++ {
			for j := start; j < len(la.Tries()); j++ {
				if willBe[i] == la.Tries()[j] {
					la.Swap(la.Tries(), start, la.Tries(), j)
					j = len(la.Tries())
					start++
				}
			}
		}
		la.Skip = len(willBe)
	} else {
		la.Skip = 0
	}
}

// GenerateNewCombinations generates cResults unique lottery forms.
// Mirrors the Java generateNewCombination(cResults, formType, willBe) method.
// Each iteration: setTries (rank by frequency), reGroup (front-load willBe),
// then recursive backtracking to find a non-winning combination.
func (la *LotteryArray) GenerateNewCombinations(cResults, formType int, willBe []int) [][]int {
	la.BuildTree(6)
	la.FormType = formType
	la.Consistance(formType)

	la.Results = make([][]int, cResults)
	for i := range la.Results {
		la.Results[i] = nil
	}

	for i := 0; i < cResults; i++ {
		la.SetTries()
		la.ReGroup(willBe)
		la.generateNewCombination(la.Skip, formType)
		la.AddResult()
	}
	return la.Results
}

// generateNewCombination is the recursive backtracking search for a
// non-winning combination. Mirrors the Java generateNewCombination(start, check).
func (la *LotteryArray) generateNewCombination(start, check int) []int {
	la.caseL = (la.caseL + 1) % 2

	if la.NotInResults() && la.AllFormsCheck() {
		return la.CutArrayTo(la.Tries(), la.FormType)
	}
	if start == la.FormType || check == len(la.Tries()) {
		return nil
	}

	la.Swap(la.Tries(), start, la.Tries(), check)
	if la.generateNewCombination(start+(la.caseL+1)%2, check+la.caseL) != nil {
		return la.CutArrayTo(la.Tries(), la.FormType)
	}
	if la.generateNewCombination(start+la.caseL, check+(la.caseL+1)%2) != nil {
		return la.CutArrayTo(la.Tries(), la.FormType)
	}
	la.Swap(la.Tries(), start, la.Tries(), check)
	return nil
}

// Tries returns the tries array (placeholder - needs implementation)
// Tries returns the current tries array. If tries hasn't been set
// (e.g. before SetTries is called), returns a default [1..37] sequence.
func (la *LotteryArray) Tries() []int {
	if la.tries != nil {
		return la.tries
	}
	tries := make([]int, 37)
	for i := 0; i < 37; i++ {
		tries[i] = i + 1
	}
	return tries
}

// SetTries populates the tries array based on TriesType.
// Mirrors the Java setTries() method.
func (la *LotteryArray) SetTries() {
	switch la.TriesType {
	case "frequent":
		la.FrequentNums("strong")
		la.Strongs = la.FrequentStrong("strong")
	case "less frequent":
		la.FrequentNums("weak")
		la.Strongs = la.FrequentStrong("weak")
	case "random":
		la.RandomCombo()
		la.Strongs = la.RandomStrong()
	}
}

// FrequentNums ranks numbers by frequency using the lottery tree.
// Mirrors the Java frequentNums(weakStrong) method.
func (la *LotteryArray) FrequentNums(weakStrong string) []int {
	count := 37
	if la.Lottery == TripleSeven {
		count = 70
	} else if la.Lottery == OneTwoThree {
		count = 10
	}

	la.BuildTree(1)
	tmp := la.Query.BestPares(count, 1, weakStrong)

	frq := make([]int, count)
	for i := 0; i < len(tmp) && i < count; i++ {
		if len(tmp[i]) > 0 {
			frq[i] = tmp[i][0]
		}
	}
	la.tries = frq
	return frq
}

// FrequentStrong ranks strong numbers (1-7) by frequency.
// Mirrors the Java frequentStrongNums(weakStrong) method.
func (la *LotteryArray) FrequentStrong(weakStrong string) []int {
	if la.Lottery != Lottery {
		return []int{}
	}
	la.Query = NewLoTree(9)
	la.Query.Build(la.StrongArchive, 1)
	tmp := la.Query.BestPares(9, 1, weakStrong)

	frq := make([]int, 7)
	ind := 0
	validIdx := 0
	for i := 0; i < len(tmp) && ind < 7; i++ {
		if len(tmp[validIdx]) > 0 && tmp[validIdx][0] < 8 {
			frq[ind] = tmp[validIdx][0]
			if frq[ind] != 0 {
				ind++
			}
		}
		validIdx++
	}
	return frq
}

// RandomCombo shuffles numbers randomly into the tries array.
// Mirrors the Java randomCombo() method.
func (la *LotteryArray) RandomCombo() []int {
	count := 37
	if la.Lottery == TripleSeven {
		count = 70
	} else if la.Lottery == OneTwoThree {
		count = 10
	}

	frq := make([]int, count)
	for j := 0; j < count; j++ {
		frq[j] = rand.Intn(count) + 1
		for i := 0; i < j; i++ {
			if frq[i] == frq[j] {
				frq[j] = (frq[j] % count) + 1
				i = -1
			}
		}
	}
	la.tries = frq
	return frq
}

// RandomStrong generates random strong numbers (1-7).
// Mirrors the Java randomStrong() method.
func (la *LotteryArray) RandomStrong() []int {
	if la.Lottery != Lottery {
		return []int{}
	}
	frq := make([]int, 7)
	for j := 0; j < 7; j++ {
		frq[j] = rand.Intn(7) + 1
		for i := 0; i < j; i++ {
			if frq[i] == frq[j] {
				frq[j] = (frq[j] % 7) + 1
				i = -1
			}
		}
	}
	return frq
}

// WinsInForm sets the wins in form counter
func (la *LotteryArray) SetWinsInForm(wif int) {
	la.WinsInForm = 0
	la.WinsInFormC = wif
}

// TooManyFollows checks if there are too many consecutive numbers
func (la *LotteryArray) TooManyFollows(lottery []int) bool {
	check := 0
	l := make([]int, len(lottery))
	copy(l, lottery)
	l = la.Sort(l)

	for i := 1; i < len(lottery); i++ {
		if l[i-1] == l[i]-1 {
			check++
		}
	}

	ok := 1
	switch len(lottery) {
	case 12:
		ok = 4
	case 11:
		ok = 3
	case 10, 9, 8:
		ok = 3
	case 7:
		ok = 2
	}

	return check > ok
}

// NotInResults checks if a combination is not in results
func (la *LotteryArray) NotInResults() bool {
	check := la.Sort(la.CutArrayTo(la.Tries(), la.FormType))
	for i := 0; i < len(la.Results); i++ {
		if la.Results[i] == nil {
			return true
		}
		if la.equalsSlices(check, la.Results[i]) {
			return false
		}
	}
	return true
}

func (la *LotteryArray) equalsSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AddResult adds a result to the results array
func (la *LotteryArray) AddResult() {
	combo := la.Sort(la.CutArrayTo(la.Tries(), la.FormType))
	finalCombo := make([]int, len(combo)+1)
	copy(finalCombo, combo)

	for i := 0; i < len(la.Results) && combo != nil; i++ {
		if la.Results[i] == nil {
			finalCombo[len(combo)] = la.Strongs[i%7]
			la.Results[i] = finalCombo
			break
		}
	}
}

// AllFormsCheck checks all forms against the tree
func (la *LotteryArray) AllFormsCheck() bool {
	la.TestCount = 0
	var newNums []int

	switch la.Lottery {
	case Lottery:
		newNums = []int{la.Tries()[0], la.Tries()[1], la.Tries()[2], la.Tries()[3], la.Tries()[4], la.Tries()[5]}
		la.Pieces = NewLotteryList()
		la.makePieces(newNums, 0, 0)
		la.Pieces.ToHeadList()
		for la.Pieces.HasMoreForms() {
			if la.WonBig(la.Pieces.NextForm(), 0) != 0 {
				return false
			}
		}
	case TripleSeven:
		newNums = []int{la.Tries()[0], la.Tries()[1], la.Tries()[2], la.Tries()[3], la.Tries()[4], la.Tries()[5], la.Tries()[6]}
		la.Pieces = NewLotteryList()
		la.makePieces(newNums, 0, 0)
		la.Pieces.ToHeadList()
		for la.Pieces.HasMoreForms() {
			if la.WonBig(la.Pieces.NextForm(), 0) != 0 {
				return false
			}
		}
	case OneTwoThree:
		newNums = []int{la.Tries()[0], la.Tries()[1], la.Tries()[2]}
	}

	return true
}

// makePieces generates all piece combinations
func (la *LotteryArray) makePieces(form []int, start, check int) {
	newNums := la.NewPointerSort(form)
	la.Pieces.Add(newNums)

	nBank := make([]int, la.FormType-la.MinNum)
	for j := la.MinNum; j < la.FormType; j++ {
		nBank[j-la.MinNum] = la.Tries()[j]
	}

	if start < len(form) && check < len(nBank) {
		la.Swap(form, start, nBank, check)
		la.makePieces(form, start, check+1)
		la.makePieces(form, start+1, 0)
		la.Swap(form, start, nBank, check)
	}
}

// AnalyzeArray analyzes a form against the tree and returns results
func (t *LoTree) AnalyzeArray(form []int, maxSize int) [][]int {
	results := make([][]int, 0)
	tmpList := []int{t.BuildCalls}
	results = append(results, tmpList)

	sequence := make([]int, 0)
	t.Anchor.analyzeBuild(form, 0, maxSize, &results, &sequence)

	return results
}

func (n *LoNode) analyzeBuild(form []int, current, howMany int, results *[][]int, sequence *[]int) {
	if howMany == 0 {
		return
	}

	for i := current; i < len(form); i++ {
		treeIndex := form[i] - n.Num - 1
		if treeIndex >= 0 && treeIndex < len(n.Next) && n.Next[treeIndex] != nil {
			*sequence = append(*sequence, form[i])
			*sequence = append(*sequence, n.Next[treeIndex].Count)

			newSeq := make([]int, len(*sequence))
			copy(newSeq, *sequence)
			*results = append(*results, newSeq)

			*sequence = (*sequence)[:len(*sequence)-1]
			n.Next[treeIndex].analyzeBuild(form, i+1, howMany-1, results, sequence)
			*sequence = (*sequence)[:len(*sequence)-1]
		}
	}
}
