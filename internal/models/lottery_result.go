package models

import "time"

type LotteryResult struct {
	ID          int       `json:"id" db:"id"`
	DrawNumber  int       `json:"draw_number" db:"draw_number"`
	DrawDate    time.Time `json:"draw_date" db:"draw_date"`
	Numbers     []int     `json:"numbers" db:"numbers"`
	Strong      int       `json:"strong" db:"strong"`
	LotteryType string    `json:"lottery_type" db:"lottery_type"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
