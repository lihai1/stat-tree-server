package seeder

import (
	"context"
	"log"

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
// A batchSize <= 0 is treated as 50.
func (ps *PrizeSeeder) SeedMissingPrizes(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 50
	}

	log.Printf("PrizeSeeder: querying for draws missing prize amounts (batchSize=%d)", batchSize)
	drawNumbers, err := ps.repo.GetDrawsWithoutPrizes(ctx, batchSize)
	if err != nil {
		log.Printf("PrizeSeeder: failed to query draws without prizes: %v", err)
		return 0, err
	}
	if len(drawNumbers) == 0 {
		log.Println("PrizeSeeder: no draws missing prize amounts")
		return 0, nil
	}

	log.Printf("PrizeSeeder: scraping prize tables for %d draws (draw %d..%d)",
		len(drawNumbers), drawNumbers[0], drawNumbers[len(drawNumbers)-1])

	written := 0
	for _, dn := range drawNumbers {
		log.Printf("PrizeSeeder: processing draw %d (%d/%d)", dn, written+1, len(drawNumbers))
		amounts, err := ps.scraper.FetchPrizeAmounts(dn)
		if err != nil {
			log.Printf("PrizeSeeder: draw %d fetch failed: %v", dn, err)
			continue
		}
		if err := ps.repo.UpdatePrizeAmounts(ctx, dn, amounts); err != nil {
			log.Printf("PrizeSeeder: failed to persist draw %d: %v", dn, err)
			continue
		}
		written++
	}

	log.Printf("PrizeSeeder: completed — wrote prize amounts for %d/%d draws", written, len(drawNumbers))
	return written, nil
}
