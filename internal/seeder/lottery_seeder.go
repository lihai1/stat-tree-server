package seeder

import (
	"context"
	"log/slog"

	"github.com/lihai1/stat-tree-server/internal/repository"
	"github.com/lihai1/stat-tree-server/internal/scraper"
)

type LotterySeeder struct {
	lotteryRepo *repository.LotteryResultRepository
	scraper     *scraper.PaisScraper
}

func NewLotterySeeder(lotteryRepo *repository.LotteryResultRepository) *LotterySeeder {
	return &LotterySeeder{
		lotteryRepo: lotteryRepo,
		scraper:     scraper.NewPaisScraper(),
	}
}

// SeedLotteryData fetches lottery data from pais.co.il and seeds the database
func (ls *LotterySeeder) SeedLotteryData(ctx context.Context) error {
	slog.Info("starting lottery data seeding...")

	// Check if we already have data
	count, err := ls.lotteryRepo.Count(ctx)
	if err != nil {
		return err
	}

	if count > 0 {
		slog.Info("database already contains lottery results, skipping seed", "count", count)
		return nil
	}

	// Fetch data from pais.co.il
	slog.Info("fetching lottery data from pais.co.il...")
	results, err := ls.scraper.FetchLotteryData()
	if err != nil {
		return err
	}

	slog.Info("fetched lottery results from pais.co.il", "count", len(results))

	// Insert into database
	slog.Info("inserting lottery results into database...")
	if err := ls.lotteryRepo.CreateBatch(ctx, results); err != nil {
		return err
	}

	slog.Info("successfully seeded lottery results", "count", len(results))
	return nil
}

// ForceSeedLotteryData forces a re-seed of lottery data (even if data exists)
func (ls *LotterySeeder) ForceSeedLotteryData(ctx context.Context) error {
	slog.Info("force seeding lottery data...")

	// Fetch data from pais.co.il
	slog.Info("fetching lottery data from pais.co.il...")
	results, err := ls.scraper.FetchLotteryData()
	if err != nil {
		return err
	}

	slog.Info("fetched lottery results from pais.co.il", "count", len(results))

	// Insert into database (will update existing records)
	slog.Info("inserting lottery results into database...")
	if err := ls.lotteryRepo.CreateBatch(ctx, results); err != nil {
		return err
	}

	slog.Info("successfully seeded lottery results", "count", len(results))
	return nil
}
