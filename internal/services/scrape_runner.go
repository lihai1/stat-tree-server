package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/lihai1/stat-tree-server/internal/models"
)

const (
	prizeBackfillBatchSize = 50   // draws per SeedMissingPrizes call
	maxPrizeBackfillPerRun = 2000 // safety cap; covers the full reachable backlog in one run
)

type LotteryDataFetcher interface {
	FetchLotteryData() ([]models.LotteryResult, error)
}

type LotteryDrawInserter interface {
	InsertNewDraws(ctx context.Context, results []models.LotteryResult) (int, time.Time, time.Time, error)
}

type CacheRangeInvalidator interface {
	InvalidateRange(affectedFrom, affectedTo time.Time)
}

type PrizeMissingSeeder interface {
	SeedMissingPrizes(ctx context.Context, batchSize int) (int, time.Time, time.Time, error)
}

type ScrapeResult struct {
	Inserted      int `json:"inserted"`
	PrizesWritten int `json:"prizes_written"`
}

type ScraperRunner struct {
	fetcher     LotteryDataFetcher
	inserter    LotteryDrawInserter
	invalidator CacheRangeInvalidator
	seeder      PrizeMissingSeeder
	mu          sync.Mutex
}

func NewScraperRunner(
	fetcher LotteryDataFetcher,
	inserter LotteryDrawInserter,
	invalidator CacheRangeInvalidator,
	seeder PrizeMissingSeeder,
) *ScraperRunner {
	return &ScraperRunner{
		fetcher:     fetcher,
		inserter:    inserter,
		invalidator: invalidator,
		seeder:      seeder,
	}
}

func (r *ScraperRunner) RunOnce(ctx context.Context, onPhase func(phase string)) (ScrapeResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var res ScrapeResult

	if onPhase != nil {
		onPhase("fetch")
	}
	results, err := r.fetcher.FetchLotteryData()
	if err != nil {
		return res, fmt.Errorf("fetch lottery data: %w", err)
	}

	if onPhase != nil {
		onPhase("insert")
	}
	inserted, fromD, toD, err := r.inserter.InsertNewDraws(ctx, results)
	if err != nil {
		return res, fmt.Errorf("insert new draws: %w", err)
	}
	res.Inserted = inserted
	slog.Info("ScraperRunner: inserted new draws", "inserted", inserted, "fetched", len(results))

	if inserted > 0 && r.invalidator != nil {
		r.invalidator.InvalidateRange(fromD, toD)
	}

	if onPhase != nil {
		onPhase("prizes")
	}
	if r.seeder != nil {
		total := 0
		for total < maxPrizeBackfillPerRun {
			written, pFrom, pTo, pErr := r.seeder.SeedMissingPrizes(ctx, prizeBackfillBatchSize)
			if pErr != nil {
				slog.Warn("ScraperRunner: prize backfill warning", "error", pErr)
				break
			}
			if written == 0 {
				break
			}
			total += written
			if r.invalidator != nil {
				r.invalidator.InvalidateRange(pFrom, pTo)
			}
		}
		res.PrizesWritten = total
		slog.Info("ScraperRunner: prize backfill complete", "written", total)
	}

	return res, nil
}
