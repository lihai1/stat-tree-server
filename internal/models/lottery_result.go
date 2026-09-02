package models

import "time"

type LotteryResult struct {
	ID           int       `json:"id" db:"id"`
	DrawNumber   int       `json:"draw_number" db:"draw_number"`
	DrawDate     time.Time `json:"draw_date" db:"draw_date"`
	Numbers      []int     `json:"numbers" db:"numbers"`
	Strong       int       `json:"strong" db:"strong"`
	LotteryType  string    `json:"lottery_type" db:"lottery_type"`
	PrizeAmounts []float64 `json:"prize_amounts,omitempty" db:"prize_amounts"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// DrawRef is a lightweight reference to a draw, carrying just the identity
// (draw_number) and the draw_date. It is used by the scraper and prize
// backfill to report which date range was affected by a write, so the
// LotteryManager can invalidate only the overlapping cache windows instead
// of clearing the whole cache.
type DrawRef struct {
	DrawNumber int
	DrawDate   time.Time
}
