package services

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/lihai1/stat-tree-server/internal/models"
	lotteryv1 "github.com/lihai1/stat-tree-server/pkg/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// errInvalidForm is returned when the simulate request has fewer than 6 numbers.
var errInvalidForm = errors.New("simulate: form must contain at least 6 numbers")

// Default prize amounts per tier (ILS) for the Israeli Lotto.
// Tiers 1-2 have published minimums; tiers 3-8 are floating and use
// conservative estimates. These are used only when a draw has no real
// prize data in the database (prize_amounts IS NULL) and the user has
// not supplied an override via SimulateRequest.prize_amounts.
//
// Real per-draw prize amounts are scraped from the unofficial paisAPI
// (see internal/scraper/prize_scraper.go) and stored in the
// lottery_results.prize_amounts JSONB column.
var defaultPrizeAmounts = [8]float64{
	5_000_000, // tier 1: 6 + strong (jackpot minimum)
	750_000,   // tier 2: 6 (second prize minimum)
	10_000,    // tier 3: 5 + strong
	2_000,     // tier 4: 5
	500,       // tier 5: 4 + strong
	150,       // tier 6: 4
	75,        // tier 7: 3 + strong
	15,        // tier 8: 3
}

// tierLabels maps tier index (1-8) to a human-readable label.
var tierLabels = [8]string{
	"6+strong", // tier 1
	"6",        // tier 2
	"5+strong", // tier 3
	"5",        // tier 4
	"4+strong", // tier 5
	"4",        // tier 6
	"3+strong", // tier 7
	"3",        // tier 8
}

// defaultTicketCost is the cost per table/combination in ILS.
const defaultTicketCost = 3.0

// Simulate backtests a user's form against historical draws over a date window.
// For systematic forms (N > 6 numbers), all C(N,6) combinations are played per
// draw. For each draw, each combination is compared to the winning numbers to
// determine the prize tier. Ticket cost and prize amounts are aggregated.
//
// Two date windows are honored:
//   - archive_window: the historical-draw range loaded from the archive.
//   - simulate_window: the sub-range of those draws to actually backtest
//     against. If unset, falls back to archive_window (legacy behavior).
//
// The cost per draw uses the actual number of combinations played
// (len(combinations)), not an artificial minimum. A six-number form has
// exactly one combination, so its per-draw cost is one ticket, not two.
func (s *LotteryService) Simulate(ctx context.Context, req *lotteryv1.SimulateRequest) (*lotteryv1.SimulateResponse, error) {
	arch, err := s.archive(ctx, req.GetArchiveWindow())
	if err != nil {
		log.Printf("Simulate: failed to load archive: %v", err)
		return nil, err
	}

	// Resolve the backtest window. If simulate_window is unset, fall back
	// to the archive window (preserving the legacy single-window behavior).
	results := arch.Draws
	simFrom, simTo := windowFromProto(req.GetSimulateWindow())
	if req.GetSimulateWindow() != nil {
		results = filterDrawsByWindow(results, simFrom, simTo)
	}

	log.Printf("Simulate: START form=%v strong=%d archiveDraws=%d simulateDraws=%d",
		req.GetForm(), req.GetStrong(), len(arch.Draws), len(results))

	// Parse user form numbers.
	form := make([]int, 0, len(req.GetForm()))
	for _, n := range req.GetForm() {
		if n > 0 {
			form = append(form, int(n))
		}
	}
	if len(form) < 6 {
		log.Printf("Simulate: rejecting form with only %d numbers (need >= 6)", len(form))
		return nil, errInvalidForm
	}

	// Deduplicate and sort the form.
	form = dedupAndSort(form)

	// Determine form size (N). Supported: 6, 8, 10, 12.
	n := len(form)
	if n > 12 {
		log.Printf("Simulate: form has %d numbers, truncating to 12", len(form))
		n = 12
		form = form[:12]
	}

	// Generate all C(N,6) combinations and use the actual count for cost.
	// A six-number form yields exactly one combination; a systematic form
	// yields C(N,6). No artificial minimum is applied.
	combinations := generateCombinations(form, 6)
	numCombinations := len(combinations)
	log.Printf("Simulate: form size=%d, combinations per draw=%d", n, numCombinations)

	// Ticket cost per draw.
	ticketCost := req.GetTicketCost()
	if ticketCost <= 0 {
		ticketCost = defaultTicketCost
	}
	costPerDraw := float64(numCombinations) * ticketCost

	// User-supplied prize-amount overrides (per tier). A value >= 0 in a
	// slot overrides both the DB value and the default for that tier.
	// nil/empty means "use real DB prizes when available, else defaults".
	var userOverrides [8]float64
	hasOverride := [8]bool{}
	if pa := req.GetPrizeAmounts(); len(pa) == 8 {
		for i := 0; i < 8; i++ {
			if pa[i] >= 0 {
				userOverrides[i] = pa[i]
				hasOverride[i] = true
			}
		}
	}

	// User's strong number (0 = no strong).
	userStrong := int(req.GetStrong())

	// Simulate each draw.
	log.Printf("Simulate: beginning per-draw simulation for %d draws", len(results))
	drawResults := make([]*lotteryv1.SimulateDrawResult, 0, len(results))
	// Aggregate per-tier stats.
	tierTotalHits := [8]int{}
	tierTotalAmount := [8]float64{}
	totalSpent := 0.0
	totalWon := 0.0
	usingRealPrizes := 0

	for _, draw := range results {
		// Resolve the effective prize amounts for this draw:
		//   user override > real DB prize > default estimate.
		drawPrizes := defaultPrizeAmounts
		drawHasReal := false
		if len(draw.PrizeAmounts) == 8 {
			drawHasReal = true
			for i := 0; i < 8; i++ {
				drawPrizes[i] = draw.PrizeAmounts[i]
			}
		}
		for i := 0; i < 8; i++ {
			if hasOverride[i] {
				drawPrizes[i] = userOverrides[i]
			}
		}
		if drawHasReal {
			usingRealPrizes++
		}

		drawResult := simulateDraw(draw, combinations, userStrong, drawPrizes, costPerDraw)
		drawResult.UsedRealPrizes = drawHasReal
		drawResults = append(drawResults, drawResult)

		totalSpent += drawResult.GetTicketCost()
		totalWon += drawResult.GetPrizeWon()
		for _, hit := range drawResult.GetTierHits() {
			idx := int(hit.GetTier()) - 1
			if idx >= 0 && idx < 8 {
				tierTotalHits[idx] += int(hit.GetHits())
				tierTotalAmount[idx] += hit.GetTotal()
			}
		}
	}

	// Filter the draw history to only include draws where the user won a prize.
	// Summary totals (TotalDraws, TotalSpent, TotalWon, Net) still reflect ALL
	// draws in the date range — only the per-draw list is trimmed.
	winningDraws := make([]*lotteryv1.SimulateDrawResult, 0, len(drawResults))
	for _, d := range drawResults {
		if d.GetPrizeWon() > 0 {
			winningDraws = append(winningDraws, d)
		}
	}
	drawResults = winningDraws

	// Build tier summaries.
	tierSummaries := make([]*lotteryv1.SimulateTierSummary, 0, 8)
	for i := 0; i < 8; i++ {
		tierSummaries = append(tierSummaries, &lotteryv1.SimulateTierSummary{
			Tier:        int32(i + 1),
			Label:       tierLabels[i],
			TotalHits:   int32(tierTotalHits[i]),
			TotalAmount: tierTotalAmount[i],
		})
	}

	summary := &lotteryv1.SimulateSummary{
		TotalDraws:          int32(len(results)),
		TotalCombinations:   int32(len(results) * numCombinations),
		TotalSpent:          totalSpent,
		TotalWon:            totalWon,
		Net:                 totalWon - totalSpent,
		TierSummaries:       tierSummaries,
		DrawsWithRealPrizes: int32(usingRealPrizes),
	}

	log.Printf("Simulate: SUCCESS totalDraws=%d totalSpent=%.2f totalWon=%.2f net=%.2f drawsUsingRealPrizes=%d/%d",
		summary.GetTotalDraws(), summary.GetTotalSpent(), summary.GetTotalWon(),
		summary.GetNet(), summary.GetDrawsWithRealPrizes(), summary.GetTotalDraws())

	return &lotteryv1.SimulateResponse{
		Draws:   drawResults,
		Summary: summary,
	}, nil
}

// simulateDraw simulates playing all combinations against a single draw.
func simulateDraw(draw models.LotteryResult, combinations [][]int, userStrong int, prizeAmounts [8]float64, costPerDraw float64) *lotteryv1.SimulateDrawResult {
	winningSet := make(map[int]bool, len(draw.Numbers))
	for _, n := range draw.Numbers {
		winningSet[n] = true
	}
	strongMatched := userStrong > 0 && userStrong == draw.Strong

	// Count hits per tier for this draw.
	tierHitsCount := [8]int{}
	for _, combo := range combinations {
		matches := 0
		for _, n := range combo {
			if winningSet[n] {
				matches++
			}
		}
		tier := matchToTier(matches, strongMatched)
		if tier > 0 {
			tierHitsCount[tier-1]++
		}
	}

	// Build tier hits (only tiers with hits > 0).
	tierHits := make([]*lotteryv1.SimulateTierHit, 0, 8)
	prizeWon := 0.0
	for i := 0; i < 8; i++ {
		if tierHitsCount[i] > 0 {
			total := float64(tierHitsCount[i]) * prizeAmounts[i]
			tierHits = append(tierHits, &lotteryv1.SimulateTierHit{
				Tier:         int32(i + 1),
				Hits:         int32(tierHitsCount[i]),
				AmountPerHit: prizeAmounts[i],
				Total:        total,
			})
			prizeWon += total
		}
	}

	return &lotteryv1.SimulateDrawResult{
		DrawNumber:     int32(draw.DrawNumber),
		DrawDate:       timestampProto(draw.DrawDate),
		WinningNumbers: utilsIntsToInt32s(draw.Numbers),
		WinningStrong:  int32(draw.Strong),
		TierHits:       tierHits,
		PrizeWon:       prizeWon,
		TicketCost:     costPerDraw,
	}
}

// matchToTier maps (matches, strongMatched) to a prize tier (1-8).
// Returns 0 if no prize was won (fewer than 3 matches).
//
//	Tier 1: 6 + strong
//	Tier 2: 6
//	Tier 3: 5 + strong
//	Tier 4: 5
//	Tier 5: 4 + strong
//	Tier 6: 4
//	Tier 7: 3 + strong
//	Tier 8: 3
func matchToTier(matches int, strongMatched bool) int {
	switch matches {
	case 6:
		if strongMatched {
			return 1
		}
		return 2
	case 5:
		if strongMatched {
			return 3
		}
		return 4
	case 4:
		if strongMatched {
			return 5
		}
		return 6
	case 3:
		if strongMatched {
			return 7
		}
		return 8
	default:
		return 0
	}
}

// generateCombinations generates all C(n, k) combinations from the given
// numbers slice. Each combination is a sorted []int of length k.
func generateCombinations(numbers []int, k int) [][]int {
	n := len(numbers)
	if k > n || k <= 0 {
		return nil
	}
	if k == n {
		return [][]int{append([]int(nil), numbers...)}
	}

	var result [][]int
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

		// Find the rightmost index that can be incremented.
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

// dedupAndSort removes duplicates and sorts the slice.
func dedupAndSort(nums []int) []int {
	seen := make(map[int]bool, len(nums))
	result := make([]int, 0, len(nums))
	for _, n := range nums {
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	sort.Ints(result)
	return result
}

// utilsIntsToInt32s is a local helper to avoid an import cycle in tests.
func utilsIntsToInt32s(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}

// timestampProto converts a time.Time to a google.protobuf.Timestamp.
func timestampProto(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

// filterDrawsByWindow returns the draws whose DrawDate falls within [from, to].
// A zero from/to means "open bound" (no lower/upper limit). Draws on the
// boundary day are inclusive. The input slice is not mutated.
func filterDrawsByWindow(draws []models.LotteryResult, from, to time.Time) []models.LotteryResult {
	if from.IsZero() && to.IsZero() {
		return draws
	}
	filtered := make([]models.LotteryResult, 0, len(draws))
	for _, d := range draws {
		if !from.IsZero() && d.DrawDate.Before(from) {
			continue
		}
		if !to.IsZero() && d.DrawDate.After(to) {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered
}
