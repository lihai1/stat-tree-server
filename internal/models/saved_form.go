package models

import "time"

type SavedForm struct {
	ID              string      `json:"id" db:"id"`
	UserID          string      `json:"user_id" db:"user_id"`
	Name            string      `json:"name" db:"name"`
	FormType        string      `json:"form_type" db:"form_type"`
	Numbers         []int32     `json:"numbers" db:"numbers"`
	ExcludeNumbers  []int32     `json:"exclude_numbers,omitempty" db:"exclude_numbers"`
	Count           *int32      `json:"count,omitempty" db:"count"`
	AnalysisType    string      `json:"analysis_type,omitempty" db:"analysis_type"`
	GeneratedResult interface{} `json:"generated_result,omitempty" db:"generated_result"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
}
