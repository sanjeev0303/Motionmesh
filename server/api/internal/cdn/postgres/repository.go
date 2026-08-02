package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motionmesh/server/api/internal/cdn"
	"github.com/motionmesh/server/shared/models"
)

type cdnRepository struct {
	db *pgxpool.Pool
}

func NewCDNRepository(db *pgxpool.Pool) cdn.Repository {
	return &cdnRepository{db: db}
}

func (r *cdnRepository) CreateDomain(ctx context.Context, domain *models.CDNDomain) error {
	query := `
		INSERT INTO cdn_domains (
			account_id, hostname, cloudflare_hostname_id, hostname_status, ssl_status, verification_errors
		) VALUES (
			$1, $2, $3, $4, $5, $6
		) RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(
		ctx, query,
		domain.AccountID,
		domain.Hostname,
		domain.CloudflareHostnameID,
		domain.HostnameStatus,
		domain.SSLStatus,
		domain.VerificationErrors,
	).Scan(&domain.ID, &domain.CreatedAt, &domain.UpdatedAt)
}

func (r *cdnRepository) GetDomain(ctx context.Context, id string) (*models.CDNDomain, error) {
	query := `
		SELECT id, account_id, hostname, cloudflare_hostname_id, hostname_status, ssl_status, verification_errors, created_at, updated_at
		FROM cdn_domains
		WHERE id = $1
	`
	var domain models.CDNDomain
	err := r.db.QueryRow(ctx, query, id).Scan(
		&domain.ID,
		&domain.AccountID,
		&domain.Hostname,
		&domain.CloudflareHostnameID,
		&domain.HostnameStatus,
		&domain.SSLStatus,
		&domain.VerificationErrors,
		&domain.CreatedAt,
		&domain.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

func (r *cdnRepository) ListDomains(ctx context.Context, accountID string) ([]models.CDNDomain, error) {
	query := `
		SELECT id, account_id, hostname, cloudflare_hostname_id, hostname_status, ssl_status, verification_errors, created_at, updated_at
		FROM cdn_domains
		WHERE account_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []models.CDNDomain
	for rows.Next() {
		var domain models.CDNDomain
		err := rows.Scan(
			&domain.ID,
			&domain.AccountID,
			&domain.Hostname,
			&domain.CloudflareHostnameID,
			&domain.HostnameStatus,
			&domain.SSLStatus,
			&domain.VerificationErrors,
			&domain.CreatedAt,
			&domain.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

func (r *cdnRepository) DeleteDomain(ctx context.Context, id string) error {
	query := `DELETE FROM cdn_domains WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func (r *cdnRepository) UpdateDomainStatus(ctx context.Context, id string, hostnameStatus, sslStatus string, verificationErrors *string) error {
	query := `
		UPDATE cdn_domains
		SET hostname_status = $1, ssl_status = $2, verification_errors = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, query, hostnameStatus, sslStatus, verificationErrors, id)
	return err
}
