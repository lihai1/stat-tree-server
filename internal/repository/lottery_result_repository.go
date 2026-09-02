package repository

import (
	"context"
	"fmt"
	"log"
	"time"

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
		INSERT INTO lottery_results (draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (draw_number) DO UPDATE
		SET draw_date = EXCLUDED.draw_date,
		    numbers = EXCLUDED.numbers,
		    strong = EXCLUDED.strong,
		    lottery_type = EXCLUDED.lottery_type,
		    prize_amounts = COALESCE(EXCLUDED.prize_amounts, lottery_results.prize_amounts),
		    updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		result.DrawNumber,
		result.DrawDate,
		result.Numbers,
		result.Strong,
		result.LotteryType,
		result.PrizeAmounts,
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

	query := `
		INSERT INTO lottery_results (draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (draw_number) DO UPDATE
		SET draw_date = EXCLUDED.draw_date,
		    numbers = EXCLUDED.numbers,
		    strong = EXCLUDED.strong,
		    lottery_type = EXCLUDED.lottery_type,
		    prize_amounts = COALESCE(EXCLUDED.prize_amounts, lottery_results.prize_amounts),
		    updated_at = EXCLUDED.updated_at
	`

	for _, result := range results {
		_, err := tx.Exec(ctx, query,
			result.DrawNumber,
			result.DrawDate,
			result.Numbers,
			result.Strong,
			result.LotteryType,
			result.PrizeAmounts,
			result.CreatedAt,
			result.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to create lottery result batch (draw %d): %w", result.DrawNumber, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *LotteryResultRepository) GetByID(ctx context.Context, id int) (*models.LotteryResult, error) {
	query := `
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at
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
		&result.PrizeAmounts,
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
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at
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
		&result.PrizeAmounts,
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
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at
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
			&result.PrizeAmounts,
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
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at
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
		&result.PrizeAmounts,
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

// GetByDateRange returns lottery results whose draw_date falls within the
// optional [from, to] window (inclusive). If both bounds are zero, all
// results are returned ordered by draw_date ascending so the tree builder
// sees chronological order. If only one bound is set, the other is open.
func (r *LotteryResultRepository) GetByDateRange(ctx context.Context, from, to time.Time) ([]models.LotteryResult, error) {
	hasFrom := !from.IsZero()
	hasTo := !to.IsZero()

	query := `
		SELECT id, draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at
		FROM lottery_results
	`
	args := make([]any, 0, 2)
	condIndex := 0
	if hasFrom || hasTo {
		query += " WHERE "
		if hasFrom {
			args = append(args, from)
			query += "draw_date >= $" + fmt.Sprintf("%d", len(args))
			condIndex++
		}
		if hasTo {
			if condIndex > 0 {
				query += " AND "
			}
			args = append(args, to)
			query += "draw_date <= $" + fmt.Sprintf("%d", len(args))
		}
	}
	query += " ORDER BY draw_date ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get lottery results by date range: %w", err)
	}
	defer rows.Close()

	var results []models.LotteryResult
	for rows.Next() {
		var result models.LotteryResult
		if err := rows.Scan(
			&result.ID,
			&result.DrawNumber,
			&result.DrawDate,
			&result.Numbers,
			&result.Strong,
			&result.LotteryType,
			&result.PrizeAmounts,
			&result.CreatedAt,
			&result.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan lottery result: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// InsertNewDraws inserts only the draws whose draw_number is not already
// present in the table. Existing rows are left untouched, so the scraper does
// not rewrite the whole history on every run. It returns how many draws were
// inserted and the [min, max] draw_date of the inserted rows, so the caller
// can invalidate only the cache windows overlapping the affected range.
// When nothing was inserted, the returned date range is zero-valued.
func (r *LotteryResultRepository) InsertNewDraws(ctx context.Context, results []models.LotteryResult) (int, time.Time, time.Time, error) {
	if len(results) == 0 {
		return 0, time.Time{}, time.Time{}, nil
	}

	// Collect the draw numbers we want to insert and ask the DB which ones
	// already exist, so we only write genuinely new rows.
	drawNumbers := make([]int, 0, len(results))
	for _, r := range results {
		drawNumbers = append(drawNumbers, r.DrawNumber)
	}
	existing, err := r.existingDrawNumbers(ctx, drawNumbers)
	if err != nil {
		return 0, time.Time{}, time.Time{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	insertQuery := `
		INSERT INTO lottery_results (draw_number, draw_date, numbers, strong, lottery_type, prize_amounts, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (draw_number) DO NOTHING
	`

	inserted := 0
	var minDate, maxDate time.Time
	for _, result := range results {
		if existing[result.DrawNumber] {
			continue
		}
		_, err := tx.Exec(ctx, insertQuery,
			result.DrawNumber,
			result.DrawDate,
			result.Numbers,
			result.Strong,
			result.LotteryType,
			result.PrizeAmounts,
			result.CreatedAt,
			result.UpdatedAt,
		)
		if err != nil {
			return 0, time.Time{}, time.Time{}, fmt.Errorf("failed to insert draw %d: %w", result.DrawNumber, err)
		}
		inserted++
		if minDate.IsZero() || result.DrawDate.Before(minDate) {
			minDate = result.DrawDate
		}
		if maxDate.IsZero() || result.DrawDate.After(maxDate) {
			maxDate = result.DrawDate
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Repository.InsertNewDraws: inserted %d/%d new draws (date range %s..%s)",
		inserted, len(results), minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))
	return inserted, minDate, maxDate, nil
}

// existingDrawNumbers returns a set of the draw numbers in candidates that
// already exist in the table.
func (r *LotteryResultRepository) existingDrawNumbers(ctx context.Context, candidates []int) (map[int]bool, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	query := `SELECT draw_number FROM lottery_results WHERE draw_number = ANY($1)`
	rows, err := r.pool.Query(ctx, query, candidates)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing draw numbers: %w", err)
	}
	defer rows.Close()

	existing := make(map[int]bool)
	for rows.Next() {
		var dn int
		if err := rows.Scan(&dn); err != nil {
			return nil, fmt.Errorf("failed to scan draw number: %w", err)
		}
		existing[dn] = true
	}
	return existing, nil
}

// UpdatePrizeAmounts updates the prize_amounts column for a single draw and
// returns the draw's draw_date, so the caller can invalidate only cache
// windows overlapping that date.
func (r *LotteryResultRepository) UpdatePrizeAmounts(ctx context.Context, drawNumber int, prizeAmounts []float64) (time.Time, error) {
	log.Printf("Repository.UpdatePrizeAmounts: updating draw %d with %d prize amounts", drawNumber, len(prizeAmounts))
	query := `
		UPDATE lottery_results
		SET prize_amounts = $2, updated_at = CURRENT_TIMESTAMP
		WHERE draw_number = $1
		RETURNING draw_date
	`
	var drawDate time.Time
	err := r.pool.QueryRow(ctx, query, drawNumber, prizeAmounts).Scan(&drawDate)
	if err != nil {
		log.Printf("Repository.UpdatePrizeAmounts: failed to update draw %d: %v", drawNumber, err)
		return time.Time{}, fmt.Errorf("failed to update prize amounts for draw %d: %w", drawNumber, err)
	}
	return drawDate, nil
}

// GetDrawsWithoutPrizes returns draw numbers that have no prize_amounts set,
// ordered by draw_number ascending. Limit caps the result count (0 = no limit).
func (r *LotteryResultRepository) GetDrawsWithoutPrizes(ctx context.Context, limit int) ([]int, error) {
	query := `SELECT draw_number FROM lottery_results WHERE prize_amounts IS NULL AND strong BETWEEN 1 AND 7 ORDER BY draw_date DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	log.Printf("Repository.GetDrawsWithoutPrizes: querying for draws with NULL prize_amounts (limit=%d)", limit)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		log.Printf("Repository.GetDrawsWithoutPrizes: query failed: %v", err)
		return nil, fmt.Errorf("failed to get draws without prizes: %w", err)
	}
	defer rows.Close()

	var drawNumbers []int
	for rows.Next() {
		var dn int
		if err := rows.Scan(&dn); err != nil {
			log.Printf("Repository.GetDrawsWithoutPrizes: row scan failed: %v", err)
			return nil, fmt.Errorf("failed to scan draw number: %w", err)
		}
		drawNumbers = append(drawNumbers, dn)
	}
	log.Printf("Repository.GetDrawsWithoutPrizes: found %d draws without prizes", len(drawNumbers))
	return drawNumbers, nil
}

// GetDrawsWithoutPrizeRefs is like GetDrawsWithoutPrizes but also returns
// each draw's date, so the prize backfill can report the affected date range
// to the cache manager for range-scoped invalidation.
func (r *LotteryResultRepository) GetDrawsWithoutPrizeRefs(ctx context.Context, limit int) ([]models.DrawRef, error) {
	query := `SELECT draw_number, draw_date FROM lottery_results WHERE prize_amounts IS NULL AND strong BETWEEN 1 AND 7 ORDER BY draw_date DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	log.Printf("Repository.GetDrawsWithoutPrizeRefs: querying for draws with NULL prize_amounts (limit=%d)", limit)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		log.Printf("Repository.GetDrawsWithoutPrizeRefs: query failed: %v", err)
		return nil, fmt.Errorf("failed to get draws without prizes: %w", err)
	}
	defer rows.Close()

	var refs []models.DrawRef
	for rows.Next() {
		var ref models.DrawRef
		if err := rows.Scan(&ref.DrawNumber, &ref.DrawDate); err != nil {
			log.Printf("Repository.GetDrawsWithoutPrizeRefs: row scan failed: %v", err)
			return nil, fmt.Errorf("failed to scan draw ref: %w", err)
		}
		refs = append(refs, ref)
	}
	log.Printf("Repository.GetDrawsWithoutPrizeRefs: found %d draws without prizes", len(refs))
	return refs, nil
}
