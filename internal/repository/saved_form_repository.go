package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lihai1/stat-tree-server/internal/models"
)

type SavedFormRepository struct {
	pool *pgxpool.Pool
}

func NewSavedFormRepository(pool *pgxpool.Pool) *SavedFormRepository {
	return &SavedFormRepository{pool: pool}
}

func (r *SavedFormRepository) Create(ctx context.Context, form *models.SavedForm) error {
	numbersJSON, err := json.Marshal(form.Numbers)
	if err != nil {
		return fmt.Errorf("failed to marshal numbers: %w", err)
	}

	var excludeNumbersJSON []byte
	if form.ExcludeNumbers != nil {
		excludeNumbersJSON, err = json.Marshal(form.ExcludeNumbers)
		if err != nil {
			return fmt.Errorf("failed to marshal exclude_numbers: %w", err)
		}
	}

	var generatedResultJSON []byte
	if form.GeneratedResult != nil {
		generatedResultJSON, err = json.Marshal(form.GeneratedResult)
		if err != nil {
			return fmt.Errorf("failed to marshal generated_result: %w", err)
		}
	}

	query := `
		INSERT INTO saved_forms (id, user_id, name, form_type, numbers, exclude_numbers, count, analysis_type, generated_result, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		form.ID,
		form.UserID,
		form.Name,
		form.FormType,
		numbersJSON,
		excludeNumbersJSON,
		form.Count,
		form.AnalysisType,
		generatedResultJSON,
		form.CreatedAt,
		form.UpdatedAt,
	).Scan(&form.ID, &form.CreatedAt, &form.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create saved form: %w", err)
	}

	return nil
}

func (r *SavedFormRepository) GetByID(ctx context.Context, id string) (*models.SavedForm, error) {
	query := `
		SELECT id, user_id, name, form_type, numbers, exclude_numbers, count, analysis_type, generated_result, created_at, updated_at
		FROM saved_forms
		WHERE id = $1
	`

	var form models.SavedForm
	var numbersJSON, excludeNumbersJSON, generatedResultJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&form.ID,
		&form.UserID,
		&form.Name,
		&form.FormType,
		&numbersJSON,
		&excludeNumbersJSON,
		&form.Count,
		&form.AnalysisType,
		&generatedResultJSON,
		&form.CreatedAt,
		&form.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("saved form not found")
		}
		return nil, fmt.Errorf("failed to get saved form: %w", err)
	}

	if err := json.Unmarshal(numbersJSON, &form.Numbers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal numbers: %w", err)
	}

	if excludeNumbersJSON != nil {
		if err := json.Unmarshal(excludeNumbersJSON, &form.ExcludeNumbers); err != nil {
			return nil, fmt.Errorf("failed to unmarshal exclude_numbers: %w", err)
		}
	}

	if generatedResultJSON != nil {
		if err := json.Unmarshal(generatedResultJSON, &form.GeneratedResult); err != nil {
			return nil, fmt.Errorf("failed to unmarshal generated_result: %w", err)
		}
	}

	return &form, nil
}

func (r *SavedFormRepository) GetByUserID(ctx context.Context, userID string) ([]*models.SavedForm, error) {
	query := `
		SELECT id, user_id, name, form_type, numbers, exclude_numbers, count, analysis_type, generated_result, created_at, updated_at
		FROM saved_forms
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved forms: %w", err)
	}
	defer rows.Close()

	var forms []*models.SavedForm
	for rows.Next() {
		var form models.SavedForm
		var numbersJSON, excludeNumbersJSON, generatedResultJSON []byte

		err := rows.Scan(
			&form.ID,
			&form.UserID,
			&form.Name,
			&form.FormType,
			&numbersJSON,
			&excludeNumbersJSON,
			&form.Count,
			&form.AnalysisType,
			&generatedResultJSON,
			&form.CreatedAt,
			&form.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saved form: %w", err)
		}

		if err := json.Unmarshal(numbersJSON, &form.Numbers); err != nil {
			return nil, fmt.Errorf("failed to unmarshal numbers: %w", err)
		}

		if excludeNumbersJSON != nil {
			if err := json.Unmarshal(excludeNumbersJSON, &form.ExcludeNumbers); err != nil {
				return nil, fmt.Errorf("failed to unmarshal exclude_numbers: %w", err)
			}
		}

		if generatedResultJSON != nil {
			if err := json.Unmarshal(generatedResultJSON, &form.GeneratedResult); err != nil {
				return nil, fmt.Errorf("failed to unmarshal generated_result: %w", err)
			}
		}

		forms = append(forms, &form)
	}

	return forms, nil
}

func (r *SavedFormRepository) Update(ctx context.Context, form *models.SavedForm) error {
	numbersJSON, err := json.Marshal(form.Numbers)
	if err != nil {
		return fmt.Errorf("failed to marshal numbers: %w", err)
	}

	var excludeNumbersJSON []byte
	if form.ExcludeNumbers != nil {
		excludeNumbersJSON, err = json.Marshal(form.ExcludeNumbers)
		if err != nil {
			return fmt.Errorf("failed to marshal exclude_numbers: %w", err)
		}
	}

	var generatedResultJSON []byte
	if form.GeneratedResult != nil {
		generatedResultJSON, err = json.Marshal(form.GeneratedResult)
		if err != nil {
			return fmt.Errorf("failed to marshal generated_result: %w", err)
		}
	}

	query := `
		UPDATE saved_forms
		SET name = $2, form_type = $3, numbers = $4, exclude_numbers = $5, count = $6, analysis_type = $7, generated_result = $8, updated_at = $9
		WHERE id = $1
		RETURNING updated_at
	`

	err = r.pool.QueryRow(ctx, query,
		form.ID,
		form.Name,
		form.FormType,
		numbersJSON,
		excludeNumbersJSON,
		form.Count,
		form.AnalysisType,
		generatedResultJSON,
		form.UpdatedAt,
	).Scan(&form.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update saved form: %w", err)
	}

	return nil
}

func (r *SavedFormRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM saved_forms WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete saved form: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("saved form not found")
	}

	return nil
}

func (r *SavedFormRepository) DeleteByUserID(ctx context.Context, userID string) error {
	query := `DELETE FROM saved_forms WHERE user_id = $1`

	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete saved forms by user: %w", err)
	}

	return nil
}
