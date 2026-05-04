package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/oops"

	"github.com/anteraja/tetra-engine/internal/domain"
)

// CartonRepository handles carton master data persistence.
type CartonRepository struct {
	db *sqlx.DB
}

// NewCartonRepository creates a new CartonRepository.
func NewCartonRepository(db *sqlx.DB) *CartonRepository {
	return &CartonRepository{db: db}
}

// UpsertCarton inserts or updates a carton by its unique code.
func (r *CartonRepository) UpsertCarton(ctx context.Context, carton *domain.Carton) error {
	query := `
		INSERT INTO cartons (code, length, width, height, max_weight, is_active, synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (code) DO UPDATE SET
			length = EXCLUDED.length,
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			max_weight = EXCLUDED.max_weight,
			is_active = EXCLUDED.is_active,
			synced_at = EXCLUDED.synced_at,
			updated_at = NOW()`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		carton.Code,
		carton.Length,
		carton.Width,
		carton.Height,
		carton.MaxWeight,
		carton.IsActive,
		&now,
	)
	if err != nil {
		return oops.
			In("carton-repository").
			With("carton_code", carton.Code).
			Wrapf(err, "upserting carton")
	}

	return nil
}

// GetActiveCartons retrieves all active cartons ordered by volume (smallest first).
func (r *CartonRepository) GetActiveCartons(ctx context.Context) ([]domain.Carton, error) {
	var cartons []domain.Carton
	err := r.db.SelectContext(ctx, &cartons,
		`SELECT * FROM cartons WHERE is_active = TRUE
		 ORDER BY (length * width * height) ASC`)
	if err != nil {
		return nil, oops.
			In("carton-repository").
			Wrapf(err, "querying active cartons")
	}
	return cartons, nil
}

// GetCartonByCode retrieves a carton by its code, returns nil if not found.
func (r *CartonRepository) GetCartonByCode(ctx context.Context, code string) (*domain.Carton, error) {
	var carton domain.Carton
	err := r.db.GetContext(ctx, &carton, "SELECT * FROM cartons WHERE code = $1", code)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, oops.
			In("carton-repository").
			With("code", code).
			Wrapf(err, "querying carton by code")
	}
	return &carton, nil
}

// GetCartonByID retrieves a carton by its ID, returns nil if not found.
func (r *CartonRepository) GetCartonByID(ctx context.Context, id int64) (*domain.Carton, error) {
	var carton domain.Carton
	err := r.db.GetContext(ctx, &carton, "SELECT * FROM cartons WHERE id = $1", id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, oops.
			In("carton-repository").
			With("id", id).
			Wrapf(err, "querying carton by ID")
	}
	return &carton, nil
}
