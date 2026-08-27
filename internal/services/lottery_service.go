package services

import (
	"context"
	"log"
	"time"

	lotterytree "github.com/lihai1/stat-tree-server/internal/lottery-tree"
	"github.com/lihai1/stat-tree-server/internal/models"
	"github.com/lihai1/stat-tree-server/internal/repository"
	"github.com/lihai1/stat-tree-server/internal/utils"
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
	willBe := utils.Int32sToInts(req.GetWillBe())

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
			numbers = utils.IntsToInt32s(row[:formType])
			strong = int32(row[formType])
		} else {
			numbers = utils.IntsToInt32s(row)
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
		forms = append(forms, &lotteryv1.NumberSet{Numbers: utils.IntsToInt32s(numbers)})
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
		resultNums := utils.IntsToInt32sFiltered(nums)
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
// The tree builds the complete AnalyzeResponse during recursion, so this
// method is a thin "load archive, strip strong, call tree, return" wrapper
// — mirroring the legacy Java FormAnalyzeCalculations.runRequest().
func (s *LotteryService) Analyze(ctx context.Context, req *lotteryv1.AnalyzeRequest) (*lotteryv1.AnalyzeResponse, error) {
	from, to := windowFromProto(req.GetWindow())
	la, _, err := s.loadArchive(ctx, from, to)
	if err != nil {
		log.Printf("Analyze: failed to load archive: %v", err)
	}

	numbers := utils.Int32sToInts(req.GetForm())
	if len(numbers) > 6 {
		numbers = numbers[:len(numbers)-1] // strip trailing strong
	}

	return la.AnalyzeForm(numbers), nil
}
