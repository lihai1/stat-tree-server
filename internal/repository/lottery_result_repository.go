package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lihai1/stat-tree-server/internal/models"
)

type LotteryResultRepository struct {
	pool *pgxpool.Pool
}

func NewLotteryResultRepository(pool *pgxpool.Pool) *LotteryResultRepository {
	return &LotteryResultRepository{pool: pool}
}

func (r *LotteryResultRepository) Create(ctx context.Context, result *models.LotteryResult) error {
	query := `
		INSERT INTO lottery_results (draw_number, draw_date, numbers, strong, lottery_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (draw_number) DO UPDATE
		SET draw_date = EXCLUDED.draw_date,
		    numbers = EXCLUDED.numbers,
		    strong = EXCLUDED.strong,
		    lottery_type = EXCLUDED.lottery_type,
		    updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		result.DrawNumber,
		result.DrawDate,
		result.Numbers,
		result.Strong,
		result.LotteryType,
		result.CreatedAt,
		result.UpdatedAt,
	).Scan(&result.ID, &result.CreatedAt, &result.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create lottery result: %w", err)
	}

	return nil
}

func (r *LotteryResultRepository) CreateBatch(ctx context.Context, results []models.LotteryResult) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, result := range results {
		if err := r.Create(ctx, &result); err != nil {
			return fmt.Errorf("failed to create lottery result batch: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *LotteryResultRepository) GetByID(ctx context.Context, id int) (*models.LotteryResult, error) {
	query := `
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, created_at, updated_at
		FROM lottery_results
		WHERE id = $1
	`

	var result models.LotteryResult
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&result.ID,
		&result.DrawNumber,
		&result.DrawDate,
		&result.Numbers,
		&result.Strong,
		&result.LotteryType,
		&result.CreatedAt,
		&result.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get lottery result: %w", err)
	}

	return &result, nil
}

func (r *LotteryResultRepository) GetByDrawNumber(ctx context.Context, drawNumber int) (*models.LotteryResult, error) {
	query := `
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, created_at, updated_at
		FROM lottery_results
		WHERE draw_number = $1
	`

	var result models.LotteryResult
	err := r.pool.QueryRow(ctx, query, drawNumber).Scan(
		&result.ID,
		&result.DrawNumber,
		&result.DrawDate,
		&result.Numbers,
		&result.Strong,
		&result.LotteryType,
		&result.CreatedAt,
		&result.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get lottery result by draw number: %w", err)
	}

	return &result, nil
}

func (r *LotteryResultRepository) GetAll(ctx context.Context, limit int) ([]models.LotteryResult, error) {
	query := `
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, created_at, updated_at
		FROM lottery_results
		ORDER BY draw_date DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get lottery results: %w", err)
	}
	defer rows.Close()

	var results []models.LotteryResult
	for rows.Next() {
		var result models.LotteryResult
		err := rows.Scan(
			&result.ID,
			&result.DrawNumber,
			&result.DrawDate,
			&result.Numbers,
			&result.Strong,
			&result.LotteryType,
			&result.CreatedAt,
			&result.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lottery result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

func (r *LotteryResultRepository) GetLatest(ctx context.Context) (*models.LotteryResult, error) {
	query := `
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, created_at, updated_at
		FROM lottery_results
		ORDER BY draw_date DESC
		LIMIT 1
	`

	var result models.LotteryResult
	err := r.pool.QueryRow(ctx, query).Scan(
		&result.ID,
		&result.DrawNumber,
		&result.DrawDate,
		&result.Numbers,
		&result.Strong,
		&result.LotteryType,
		&result.CreatedAt,
		&result.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get latest lottery result: %w", err)
	}

	return &result, nil
}

func (r *LotteryResultRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM lottery_results`

	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count lottery results: %w", err)
	}

	return count, nil
}
