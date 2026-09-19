package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type ScraperConsumer struct {
	rdb    redis.Cmdable
	runner *ScraperRunner
	stopCh chan struct{}
	doneCh chan struct{}
}

func NewScraperConsumer(rdb redis.Cmdable, runner *ScraperRunner) *ScraperConsumer {
	return &ScraperConsumer{
		rdb:    rdb,
		runner: runner,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (c *ScraperConsumer) DrainPending(ctx context.Context) {
	streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    "scraper-workers",
		Consumer: "lottery-worker-1",
		Streams:  []string{"scraper:requests", "0"},
		Count:    100,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		slog.Warn("ScraperConsumer: DrainPending error", "error", err)
		return
	}
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			reqID, _ := msg.Values["request_id"].(string)
			if reqID == "" {
				reqID, _ = msg.Values["id"].(string)
			}
			if reqID != "" {
				slog.Warn("ScraperConsumer: marking crashed request as interrupted", "request_id", reqID)
				statusKey := fmt.Sprintf("scraper:status:%s", reqID)
				c.rdb.HSet(ctx, statusKey,
					"status", "interrupted",
					"error", "interrupted by server restart",
					"updated_at", time.Now().Unix(),
				)
				c.rdb.Expire(ctx, statusKey, 24*time.Hour)
			}
			c.rdb.XAck(ctx, "scraper:requests", "scraper-workers", msg.ID)
		}
	}
}

func (c *ScraperConsumer) Start() {
	ctx := context.Background()
	err := c.rdb.XGroupCreateMkStream(ctx, "scraper:requests", "scraper-workers", "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		slog.Warn("ScraperConsumer: XGroupCreateMkStream", "error", err)
	}

	c.DrainPending(ctx)

	go c.loop()
}

func (c *ScraperConsumer) loop() {
	defer close(c.doneCh)
	consumerName := "lottery-worker-1"

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		streams, err := c.rdb.XReadGroup(context.Background(), &redis.XReadGroupArgs{
			Group:    "scraper-workers",
			Consumer: consumerName,
			Streams:  []string{"scraper:requests", ">"},
			Count:    1,
			Block:    2 * time.Second,
		}).Result()

		if err != nil {
			if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "context canceled") {
				continue
			}
			select {
			case <-c.stopCh:
				return
			default:
				time.Sleep(500 * time.Millisecond)
				continue
			}
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				c.ProcessMessage(msg)
			}
		}
	}
}

func (c *ScraperConsumer) ProcessMessage(msg redis.XMessage) {
	ctx := context.Background()
	reqID, _ := msg.Values["request_id"].(string)
	if reqID == "" {
		reqID, _ = msg.Values["id"].(string)
	}
	if reqID == "" {
		reqID = msg.ID
	}

	c.rdb.Set(ctx, "scraper:latest", reqID, 24*time.Hour)
	statusKey := fmt.Sprintf("scraper:status:%s", reqID)
	eventsKey := fmt.Sprintf("scraper:events:%s", reqID)

	c.rdb.HSet(ctx, statusKey,
		"request_id", reqID,
		"status", "running",
		"phase", "queued",
		"updated_at", time.Now().Unix(),
	)
	c.rdb.Expire(ctx, statusKey, 24*time.Hour)

	onPhase := func(phase string) {
		now := time.Now().Unix()
		c.rdb.HSet(ctx, statusKey,
			"phase", phase,
			"updated_at", now,
		)
		c.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: eventsKey,
			MaxLen: 1000,
			Approx: true,
			Values: map[string]interface{}{
				"event": "progress",
				"phase": phase,
				"ts":    now,
			},
		})
		c.rdb.Expire(ctx, eventsKey, 1*time.Hour)
	}

	res, err := c.runner.RunOnce(ctx, onPhase)
	now := time.Now().Unix()
	if err != nil {
		slog.Error("ScraperConsumer: RunOnce failed", "request_id", reqID, "error", err)
		c.rdb.HSet(ctx, statusKey,
			"status", "error",
			"phase", "error",
			"error", err.Error(),
			"updated_at", now,
		)
		c.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: eventsKey,
			MaxLen: 1000,
			Approx: true,
			Values: map[string]interface{}{
				"event":   "error",
				"message": err.Error(),
				"ts":      now,
			},
		})
	} else {
		c.rdb.HSet(ctx, statusKey,
			"status", "done",
			"phase", "completed",
			"inserted", res.Inserted,
			"prizes_written", res.PrizesWritten,
			"updated_at", now,
		)
		c.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: eventsKey,
			MaxLen: 1000,
			Approx: true,
			Values: map[string]interface{}{
				"event":          "done",
				"inserted":       res.Inserted,
				"prizes_written": res.PrizesWritten,
				"ts":             now,
			},
		})
	}
	c.rdb.Expire(ctx, statusKey, 24*time.Hour)
	c.rdb.Expire(ctx, eventsKey, 1*time.Hour)
	c.rdb.XAck(ctx, "scraper:requests", "scraper-workers", msg.ID)
}

func (c *ScraperConsumer) Stop() {
	close(c.stopCh)
	<-c.doneCh
}
