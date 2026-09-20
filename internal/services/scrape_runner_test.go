package services_test

import (
	"context"
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lihai1/stat-tree-server/internal/models"
	"github.com/lihai1/stat-tree-server/internal/services"
)

type mockFetcher struct {
	results []models.LotteryResult
	err     error
}

func (m *mockFetcher) FetchLotteryData() ([]models.LotteryResult, error) {
	return m.results, m.err
}

type mockInserter struct {
	inserted int
	from     time.Time
	to       time.Time
	err      error
}

func (m *mockInserter) InsertNewDraws(ctx context.Context, results []models.LotteryResult) (int, time.Time, time.Time, error) {
	return m.inserted, m.from, m.to, m.err
}

type mockInvalidator struct {
	calls int
	froms []time.Time
	tos   []time.Time
}

func (m *mockInvalidator) InvalidateRange(from, to time.Time) {
	m.calls++
	m.froms = append(m.froms, from)
	m.tos = append(m.tos, to)
}

type seederResult struct {
	written int
	from    time.Time
	to      time.Time
	err     error
}

type mockSeeder struct {
	results []seederResult // consumed in order; last result repeats if exhausted
	calls   int
}

func (m *mockSeeder) SeedMissingPrizes(ctx context.Context, batchSize int) (int, time.Time, time.Time, error) {
	m.calls++
	if len(m.results) == 0 {
		return 0, time.Time{}, time.Time{}, nil
	}
	r := m.results[0]
	if len(m.results) > 1 {
		m.results = m.results[1:]
	}
	return r.written, r.from, r.to, r.err
}

var _ = Describe("ScraperRunner", func() {
	It("should execute all phases and return inserted and prizes written", func() {
		fetcher := &mockFetcher{
			results: []models.LotteryResult{
				{DrawNumber: 3000, Numbers: []int{1, 2, 3, 4, 5, 6}, Strong: 1},
				{DrawNumber: 3001, Numbers: []int{7, 8, 9, 10, 11, 12}, Strong: 2},
			},
		}
		fromD := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		toD := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		inserter := &mockInserter{inserted: 2, from: fromD, to: toD}
		invalidator := &mockInvalidator{}
		seeder := &mockSeeder{results: []seederResult{
			{written: 5, from: fromD, to: toD},
			{written: 0},
		}}

		runner := services.NewScraperRunner(fetcher, inserter, invalidator, seeder)

		var phases []string
		res, err := runner.RunOnce(context.Background(), func(p string) {
			phases = append(phases, p)
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(res.Inserted).To(Equal(2))
		Expect(res.PrizesWritten).To(Equal(5))
		Expect(phases).To(Equal([]string{"fetch", "insert", "prizes"}))
		Expect(invalidator.calls).To(Equal(2))
	})

	It("should drain prizes across multiple batches until empty", func() {
		fetcher := &mockFetcher{results: []models.LotteryResult{{DrawNumber: 3000}}}
		inserter := &mockInserter{inserted: 1}
		invalidator := &mockInvalidator{}
		d1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		d2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		d3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		seeder := &mockSeeder{results: []seederResult{
			{written: 50, from: d1, to: d1},
			{written: 50, from: d2, to: d2},
			{written: 12, from: d3, to: d3},
			{written: 0},
		}}

		runner := services.NewScraperRunner(fetcher, inserter, invalidator, seeder)
		res, err := runner.RunOnce(context.Background(), nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.PrizesWritten).To(Equal(112))
		Expect(seeder.calls).To(Equal(4))
		Expect(invalidator.calls).To(Equal(4)) // 1 for insert + 3 batch ranges
		Expect(invalidator.tos).To(Equal([]time.Time{time.Time{}, d1, d2, d3}))
	})

	It("should stop prize drain on error and keep best-effort", func() {
		fetcher := &mockFetcher{results: []models.LotteryResult{{DrawNumber: 3000}}}
		inserter := &mockInserter{inserted: 1}
		seeder := &mockSeeder{results: []seederResult{
			{written: 50},
			{err: errors.New("pais unreachable")},
		}}

		runner := services.NewScraperRunner(fetcher, inserter, nil, seeder)
		res, err := runner.RunOnce(context.Background(), nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.PrizesWritten).To(Equal(50))
		Expect(seeder.calls).To(Equal(2))
	})

	It("should cap prize drain at maxPrizeBackfillPerRun", func() {
		fetcher := &mockFetcher{results: []models.LotteryResult{{DrawNumber: 3000}}}
		inserter := &mockInserter{inserted: 1}
		seeder := &mockSeeder{results: []seederResult{{written: 50}}}

		runner := services.NewScraperRunner(fetcher, inserter, nil, seeder)
		res, err := runner.RunOnce(context.Background(), nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(res.PrizesWritten).To(Equal(2000))
		Expect(seeder.calls).To(Equal(40))
	})

	It("should handle fetcher error cleanly", func() {
		fetcher := &mockFetcher{err: errors.New("network timeout")}
		runner := services.NewScraperRunner(fetcher, &mockInserter{}, nil, nil)

		_, err := runner.RunOnce(context.Background(), nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("network timeout"))
	})

	It("should serialize concurrent runs safely with mutex", func() {
		fetcher := &mockFetcher{results: []models.LotteryResult{{DrawNumber: 1}}}
		inserter := &mockInserter{inserted: 1}
		runner := services.NewScraperRunner(fetcher, inserter, nil, nil)

		var wg sync.WaitGroup
		results := make([]services.ScrapeResult, 5)
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				res, err := runner.RunOnce(context.Background(), nil)
				Expect(err).ToNot(HaveOccurred())
				results[idx] = res
			}(i)
		}
		wg.Wait()

		for _, r := range results {
			Expect(r.Inserted).To(Equal(1))
		}
	})
})
