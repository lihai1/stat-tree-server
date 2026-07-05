package repository

import (
	"context"
	"github.com/google/uuid"
)

// BaseRepository defines common CRUD operations
type BaseRepository[T any] interface {
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id uuid.UUID) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uuid.UUID) error
}
