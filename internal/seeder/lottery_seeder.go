package seeder

import (
	"context"
	"log"

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
	log.Println("Starting lottery data seeding...")

	// Check if we already have data
	count, err := ls.lotteryRepo.Count(ctx)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Printf("Database already contains %d lottery results. Skipping seed.", count)
		return nil
	}

	// Fetch data from pais.co.il
	log.Println("Fetching lottery data from pais.co.il...")
	results, err := ls.scraper.FetchLotteryData()
	if err != nil {
		return err
	}

	log.Printf("Fetched %d lottery results from pais.co.il", len(results))

	// Insert into database
	log.Println("Inserting lottery results into database...")
	if err := ls.lotteryRepo.CreateBatch(ctx, results); err != nil {
		return err
	}

	log.Printf("Successfully seeded %d lottery results", len(results))
	return nil
}

// ForceSeedLotteryData forces a re-seed of lottery data (even if data exists)
func (ls *LotterySeeder) ForceSeedLotteryData(ctx context.Context) error {
	log.Println("Force seeding lottery data...")

	// Fetch data from pais.co.il
	log.Println("Fetching lottery data from pais.co.il...")
	results, err := ls.scraper.FetchLotteryData()
	if err != nil {
		return err
	}

	log.Printf("Fetched %d lottery results from pais.co.il", len(results))

	// Insert into database (will update existing records)
	log.Println("Inserting lottery results into database...")
	if err := ls.lotteryRepo.CreateBatch(ctx, results); err != nil {
		return err
	}

	log.Printf("Successfully seeded %d lottery results", len(results))
	return nil
}
