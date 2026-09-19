package services_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/redis/go-redis/v9"

	"github.com/alicebob/miniredis/v2"
	"github.com/lihai1/stat-tree-server/internal/models"
	"github.com/lihai1/stat-tree-server/internal/services"
)

var _ = Describe("ScraperConsumer", func() {
	var (
		mr     *miniredis.Miniredis
		rdb    *redis.Client
		runner *services.ScraperRunner
	)

	BeforeEach(func() {
		var err error
		mr, err = miniredis.Run()
		Expect(err).ToNot(HaveOccurred())

		rdb = redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		fetcher := &mockFetcher{
			results: []models.LotteryResult{
				{DrawNumber: 3000, Numbers: []int{1, 2, 3, 4, 5, 6}, Strong: 1},
			},
		}
		inserter := &mockInserter{inserted: 1}
		runner = services.NewScraperRunner(fetcher, inserter, nil, nil)
	})

	AfterEach(func() {
		if rdb != nil {
			_ = rdb.Close()
		}
		if mr != nil {
			mr.Close()
		}
	})

	It("ProcessMessage should execute runner and update status and events", func() {
		consumer := services.NewScraperConsumer(rdb, runner)

		msg := redis.XMessage{
			ID: "1-0",
			Values: map[string]interface{}{
				"request_id": "req-123",
				"source":     "ui",
			},
		}

		consumer.ProcessMessage(msg)

		ctx := context.Background()
		status, err := rdb.HGetAll(ctx, "scraper:status:req-123").Result()
		Expect(err).ToNot(HaveOccurred())
		Expect(status["status"]).To(Equal("done"))
		Expect(status["inserted"]).To(Equal("1"))

		latest, err := rdb.Get(ctx, "scraper:latest").Result()
		Expect(err).ToNot(HaveOccurred())
		Expect(latest).To(Equal("req-123"))
	})

	It("DrainPending should mark unacked crashed requests as interrupted", func() {
		consumer := services.NewScraperConsumer(rdb, runner)
		ctx := context.Background()

		_ = rdb.XGroupCreateMkStream(ctx, "scraper:requests", "scraper-workers", "0").Err()
		msgID, err := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: "scraper:requests",
			Values: map[string]interface{}{
				"request_id": "crashed-456",
			},
		}).Result()
		Expect(err).ToNot(HaveOccurred())

		// Read it into the consumer group without acking so it is pending
		_, _ = rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    "scraper-workers",
			Consumer: "lottery-worker-1",
			Streams:  []string{"scraper:requests", ">"},
			Count:    1,
		}).Result()

		// Run DrainPending
		consumer.DrainPending(ctx)

		status, err := rdb.HGetAll(ctx, "scraper:status:crashed-456").Result()
		Expect(err).ToNot(HaveOccurred())
		Expect(status["status"]).To(Equal("interrupted"))
		Expect(status["error"]).To(ContainSubstring("interrupted"))
		_ = msgID
	})
})
