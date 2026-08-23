package services

import (
	"context"
	"log"
	"sort"
	"time"

	lotterytree "github.com/lihai1/stat-tree-server/internal/lottery-tree"
	"github.com/lihai1/stat-tree-server/internal/models"
	"github.com/lihai1/stat-tree-server/internal/repository"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
)

const version = "1.0.0"

// LotteryService implements the lotteryv1.LotteryServiceServer interface.
// It wraps the tree-based LotteryArray analysis engine (internal/lottery-tree)
// and exposes it over gRPC. The service is stateless beyond the
// lottery_results table owned by the repository layer: each RPC loads the
// historical archive fresh from the database into a new LotteryArray.
type LotteryService struct {
	lotteryv1.UnimplementedLotteryServiceServer
	repo *repository.LotteryResultRepository
}

// NewLotteryService creates a new LotteryService instance wired to the
// lottery_results repository. The repository is required so that each RPC
// can load the historical archive; without it the tree would be empty.
func NewLotteryService(repo *repository.LotteryResultRepository) *LotteryService {
	return &LotteryService{
		repo: repo,
	}
}

// loadArchive fetches historical draws within the optional [from, to] window
// and populates a fresh LotteryArray's Archive ([][]int of the 6 regular
// numbers), StrongArchive ([][]int{strong}), Strongs, and Results slots.
// The raw results are also returned so callers (e.g. Analyze) can access
// draw metadata (draw number, draw date) that the LotteryArray itself does
// not store. The LotteryArray is created per-call so the service stays
// stateless.
func (s *LotteryService) loadArchive(ctx context.Context, from, to time.Time) (*lotterytree.LotteryArray, []models.LotteryResult, error) {
	la := lotterytree.NewLotteryArray("", lotterytree.Lottery)

	if s.repo == nil {
		return la, nil, nil // graceful degradation for tests without a DB
	}

	results, err := s.repo.GetByDateRange(ctx, from, to)
	if err != nil {
		return nil, nil, err
	}

	la.Archive = make([][]int, 0, len(results))
	la.StrongArchive = make([][]int, 0, len(results))
	la.Strongs = make([]int, 0, len(results))
	la.Results = make([][]int, len(results)) // pre-allocated slots for AddResult

	for _, r := range results {
		la.Archive = append(la.Archive, r.Numbers)
		la.StrongArchive = append(la.StrongArchive, []int{r.Strong})
		la.Strongs = append(la.Strongs, r.Strong)
	}

	return la, results, nil
}

// windowFromProto converts the proto DateWindow into a (from, to) pair of
// time.Time. Zero values mean "open bound".
func windowFromProto(w *lotteryv1.DateWindow) (time.Time, time.Time) {
	var from, to time.Time
	if w != nil {
		if ts := w.GetFrom(); ts != nil {
			from = ts.AsTime()
		}
		if ts := w.GetTo(); ts != nil {
			to = ts.AsTime()
		}
	}
	return from, to
}

// HealthCheck returns the health status of the service.
func (s *LotteryService) HealthCheck(ctx context.Context, req *lotteryv1.HealthCheckRequest) (*lotteryv1.HealthCheckResponse, error) {
	drawsLoaded := int32(0)
	if s.repo != nil {
		if count, err := s.repo.Count(ctx); err == nil {
			drawsLoaded = int32(count)
		}
	}
	return &lotteryv1.HealthCheckResponse{
		Status:      "healthy",
		Version:     version,
		DrawsLoaded: drawsLoaded,
	}, nil
}

// GenerateForm generates lottery number combinations based on input parameters.
// Uses the full lottery-tree algorithm: frequency-ranked tries, ReGroup for
// lucky numbers (willBe), recursive backtracking to find non-winning combos,
// and strong-number selection. Returns howMany forms as requested by the UI.
func (s *LotteryService) GenerateForm(ctx context.Context, req *lotteryv1.GenerateFormRequest) (*lotteryv1.GenerateFormResponse, error) {
	howMany := int(req.GetHowMany())
	if howMany <= 0 {
		howMany = 1
	}
	if howMany > 50 {
		howMany = 50
	}

	from, to := windowFromProto(req.GetWindow())
	la, _, err := s.loadArchive(ctx, from, to)
	if err != nil {
		log.Printf("GenerateForm: failed to load archive: %v", err)
	}

	// Set strength mode for tries ranking.
	switch req.GetStrength() {
	case lotteryv1.Strength_WEAK:
		la.SetLessFrequentCombo()
	case lotteryv1.Strength_STRONG:
		la.SetFrequentCombo()
	default:
		la.SetFrequentCombo()
	}

	// Form type / systematic size (default 6 = regular lotto).
	formType := int(req.GetFormType())
	if formType < 6 {
		formType = 6
	}

	// Lucky numbers (willBe) to front-load.
	willBe := make([]int, len(req.GetWillBe()))
	for i, n := range req.GetWillBe() {
		willBe[i] = int(n)
	}

	// Generate howMany forms using the full algorithm.
	results := la.GenerateNewCombinations(howMany, formType, willBe)

	// Build proto response. AddResult appends strong as the last element
	// of each result row, so we split it out into NumberSet.strong.
	forms := make([]*lotteryv1.NumberSet, 0, len(results))
	for _, row := range results {
		if row == nil {
			continue
		}
		// The last element is the strong number (if present).
		var numbers []int32
		var strong int32
		if len(row) > formType {
			numbers = make([]int32, formType)
			for i := 0; i < formType; i++ {
				numbers[i] = int32(row[i])
			}
			strong = int32(row[formType])
		} else {
			numbers = make([]int32, len(row))
			for i, n := range row {
				numbers[i] = int32(n)
			}
		}
		ns := &lotteryv1.NumberSet{Numbers: numbers}
		if strong > 0 {
			ns.Strong = &strong
		}
		forms = append(forms, ns)
	}

	// Fallback: if no forms were generated (all variants have won),
	// produce at least one form from the frequency-ranked tries.
	if len(forms) == 0 {
		la.SetTries()
		la.ReGroup(willBe)
		numbers := la.Sort(la.CutArrayTo(la.Tries(), formType))
		nums32 := make([]int32, len(numbers))
		for i, n := range numbers {
			nums32[i] = int32(n)
		}
		forms = append(forms, &lotteryv1.NumberSet{Numbers: nums32})
	}

	return &lotteryv1.GenerateFormResponse{
		Forms: forms,
	}, nil
}

// GetStatistics calculates statistics for number pairs/groups.
// Preserves the original algorithm behavior: builds the tree and computes
// repeating pairs. Wires formType as the group size and strength as the
// strong/weak mode. The count (last element of each RepeatingPares row) is
// correctly split into Pair.Count.
func (s *LotteryService) GetStatistics(ctx context.Context, req *lotteryv1.GetStatisticsRequest) (*lotteryv1.GetStatisticsResponse, error) {
	from, to := windowFromProto(req.GetWindow())
	la, _, err := s.loadArchive(ctx, from, to)
	if err != nil {
		log.Printf("GetStatistics: failed to load archive: %v", err)
	}

	// Build the lottery tree for analysis (original behavior used 6).
	la.BuildTree(6)

	// howMany = number of pairs/groups to return (default 10).
	howMany := int(req.GetHowMany())
	if howMany <= 0 {
		howMany = 10
	}

	// formType = group size (default 2 = pairs, matching original behavior).
	groupSize := int(req.GetFormType())
	if groupSize <= 0 {
		groupSize = 2
	}

	// strength mode (default strong).
	strength := string(lotterytree.Strong)
	switch req.GetStrength() {
	case lotteryv1.Strength_WEAK:
		strength = string(lotterytree.Weak)
	case lotteryv1.Strength_STRONG:
		strength = string(lotterytree.Strong)
	}

	pares := la.RepeatingPares(howMany, groupSize, strength)

	pairs := make([]*lotteryv1.Pair, 0, len(pares))
	for _, row := range pares {
		if len(row) == 0 {
			continue
		}
		// Each row is [num1, num2, ..., numN, count]. The last element is the
		// occurrence count; the rest are the numbers in the group.
		count := int32(0)
		nums := row
		if len(row) > 1 {
			count = int32(row[len(row)-1])
			nums = row[:len(row)-1]
		}
		resultNums := make([]int32, 0, len(nums))
		for _, num := range nums {
			if num > 0 {
				resultNums = append(resultNums, int32(num))
			}
		}
		if len(resultNums) > 0 {
			pairs = append(pairs, &lotteryv1.Pair{
				Numbers: resultNums,
				Count:   count,
			})
		}
	}

	return &lotteryv1.GetStatisticsResponse{
		Pairs: pairs,
	}, nil
}

// Analyze analyzes user-selected numbers against historical data.
// Preserves the original algorithm behavior: builds the tree, analyzes the
// form, and returns grouped frequency results.
//
// The tree's AnalyzeArray returns a 2D slice where:
//   - results[0] = [BuildCalls] (metadata: archive size / build call count)
//   - results[1:] = interleaved [num, count, num, count, ...] sequences
//
// Each sequence's length / 2 gives the group size (1=single, 2=pair, etc.).
// We group sequences by size, extract the numbers (even indices) and the
// occurrence count (last element), and build FrequencyGroup messages that
// mirror the legacy ArraysFilter pipe output.
func (s *LotteryService) Analyze(ctx context.Context, req *lotteryv1.AnalyzeRequest) (*lotteryv1.AnalyzeResponse, error) {
	from, to := windowFromProto(req.GetWindow())
	la, _, err := s.loadArchive(ctx, from, to)
	if err != nil {
		log.Printf("Analyze: failed to load archive: %v", err)
	}

	// Convert request numbers to int slice.
	numbers := make([]int, len(req.GetForm()))
	for i, num := range req.GetForm() {
		numbers[i] = int(num)
	}

	// Defensive: if the form includes a trailing strong number (>6 regular
	// numbers for lotto), drop the last element to match the legacy server
	// behavior (FormAnalyzeCalculations.java used form.length-1).
	// The UI already strips strong, but API callers may include it.
	if len(numbers) > 6 {
		numbers = numbers[:len(numbers)-1]
	}

	// Build the lottery tree for analysis (original behavior used 6).
	la.BuildTree(6)

	// Analyze the form using the tree → 2D result array.
	analysisResults := la.AnalyzeForm(numbers)

	// Extract archive size from the metadata row (results[0] = [BuildCalls]).
	archiveSize := int32(0)
	if len(analysisResults) > 0 && len(analysisResults[0]) > 0 {
		archiveSize = int32(analysisResults[0][0])
	}

	// Process frequency rows (results[1:]) and group by sequence length.
	// Each row is [num1, count1, num2, count2, ...]; group size = len(row)/2.
	groupMap := make(map[int][]*lotteryv1.FrequencyEntry)
	for i := 1; i < len(analysisResults); i++ {
		row := analysisResults[i]
		if len(row) < 2 || len(row)%2 != 0 {
			continue
		}
		groupSize := len(row) / 2

		// Numbers are at even indices; count is the last element.
		nums := make([]int32, 0, groupSize)
		for j := 0; j < len(row)-1; j += 2 {
			if row[j] > 0 {
				nums = append(nums, int32(row[j]))
			}
		}
		count := int32(row[len(row)-1])
		if len(nums) == 0 {
			continue
		}

		groupMap[groupSize] = append(groupMap[groupSize], &lotteryv1.FrequencyEntry{
			Numbers: nums,
			Count:   count,
		})
	}

	// Build FrequencyGroup list for sizes 1–6, sorted by count descending.
	frequencyGroups := make([]*lotteryv1.FrequencyGroup, 0, 6)
	for size := 1; size <= 6; size++ {
		entries := groupMap[size]
		sort.SliceStable(entries, func(a, b int) bool {
			return entries[a].GetCount() > entries[b].GetCount()
		})
		frequencyGroups = append(frequencyGroups, &lotteryv1.FrequencyGroup{
			Size:    int32(size),
			Combos:  int32(binomial(37, size)),
			Entries: entries,
		})
	}

	return &lotteryv1.AnalyzeResponse{
		FrequencyGroups: frequencyGroups,
		ArchiveSize:     archiveSize,
	}, nil
}

// binomial returns C(n, k), the number of k-element combinations from an
// n-element set. Used to compute the total possible combinations per group
// size, matching the legacy sumCombinations helper.
func binomial(n, k int) int {
	if k < 0 || k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	k = min(k, n-k)
	result := 1
	for i := 0; i < k; i++ {
		result = result * (n - i) / (i + 1)
	}
	return result
}
