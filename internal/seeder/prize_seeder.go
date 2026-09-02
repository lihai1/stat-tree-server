package seeder

import (
	"context"
	"log"
	"time"

	"github.com/lihai1/stat-tree-server/internal/repository"
	"github.com/lihai1/stat-tree-server/internal/scraper"
)

// PrizeSeeder backfills the prize_amounts column on lottery_results by
// scraping the per-draw prize table from the pais.co.il individual draw
// pages (currentlotto.aspx?lotteryId=N).
//
// It is intentionally best-effort: if a draw page is unreachable or the
// prize table cannot be parsed, the corresponding row keeps NULL
// prize_amounts and the simulation service falls back to its default
// prize amounts.
type PrizeSeeder struct {
	repo    *repository.LotteryResultRepository
	scraper *scraper.PaisScraper
}

// NewPrizeSeeder returns a PrizeSeeder wired to the given repository.
func NewPrizeSeeder(repo *repository.LotteryResultRepository) *PrizeSeeder {
	return &PrizeSeeder{
		repo:    repo,
		scraper: scraper.NewPaisScraper(),
	}
}

// SeedMissingPrizes fetches prize amounts for up to batchSize draws that
// have NULL prize_amounts, in ascending draw-number order. Each draw is
// scraped individually from pais.co.il. Safe to call repeatedly.
//
// A batchSize <= 0 is treated as 50. It returns the number of draws written
// and the [min, max] draw_date of the updated rows, so the caller can
// invalidate only the cache windows overlapping the affected range. When
// nothing was written the date range is zero-valued.
func (ps *PrizeSeeder) SeedMissingPrizes(ctx context.Context, batchSize int) (int, time.Time, time.Time, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	log.Printf("PrizeSeeder: querying for draws missing prize amounts (batchSize=%d)", batchSize)
	refs, err := ps.repo.GetDrawsWithoutPrizeRefs(ctx, batchSize)
	if err != nil {
		log.Printf("PrizeSeeder: failed to query draws without prizes: %v", err)
		return 0, time.Time{}, time.Time{}, err
	}
	if len(refs) == 0 {
		log.Println("PrizeSeeder: no draws missing prize amounts")
		return 0, time.Time{}, time.Time{}, nil
	}

	log.Printf("PrizeSeeder: scraping prize tables for %d draws (draw %d..%d)",
		len(refs), refs[0].DrawNumber, refs[len(refs)-1].DrawNumber)

	written := 0
	var minDate, maxDate time.Time
	for _, ref := range refs {
		dn := ref.DrawNumber
		log.Printf("PrizeSeeder: processing draw %d (%d/%d)", dn, written+1, len(refs))
		amounts, err := ps.scraper.FetchPrizeAmounts(dn)
		if err != nil {
			log.Printf("PrizeSeeder: draw %d fetch failed: %v", dn, err)
			continue
		}
		drawDate, err := ps.repo.UpdatePrizeAmounts(ctx, dn, amounts)
		if err != nil {
			log.Printf("PrizeSeeder: failed to persist draw %d: %v", dn, err)
			continue
		}
		written++
		if !drawDate.IsZero() {
			if minDate.IsZero() || drawDate.Before(minDate) {
				minDate = drawDate
			}
			if maxDate.IsZero() || drawDate.After(maxDate) {
				maxDate = drawDate
			}
		}
	}

	log.Printf("PrizeSeeder: completed — wrote prize amounts for %d/%d draws (date range %s..%s)",
		written, len(refs), minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))
	return written, minDate, maxDate, nil
}
