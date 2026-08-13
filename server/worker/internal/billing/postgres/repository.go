package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motionmesh/server/shared/models"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetAccountByID(ctx context.Context, id string) (*models.Account, error) {
	var acc models.Account
	err := r.db.QueryRow(ctx,
		`SELECT id, email, clerk_user_id, clerk_org_id, stripe_customer_id, plan, status, balance, created_at, updated_at, total_storage_bytes, total_videos
		 FROM accounts WHERE id = $1`,
		id,
	).Scan(&acc.ID, &acc.Email, &acc.ClerkUserID, &acc.ClerkOrgID, &acc.StripeCustomerID, &acc.Plan, &acc.Status, &acc.Balance, &acc.CreatedAt, &acc.UpdatedAt, &acc.TotalStorageBytes, &acc.TotalVideos)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &acc, err
}

func (r *Repository) RecordUsageEvent(ctx context.Context, event *models.UsageEvent) error {
	var id *string
	if event.ID != "" {
		id = &event.ID
	}
	
	var createdAt *time.Time
	if !event.CreatedAt.IsZero() {
		createdAt = &event.CreatedAt
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO usage_events (id, account_id, event_type, quantity, metadata, created_at)
		 VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, COALESCE($6, now()))
		 ON CONFLICT (id) DO NOTHING`,
		id, event.AccountID, event.EventType, event.Quantity, event.Metadata, createdAt,
	)
	return err
}
