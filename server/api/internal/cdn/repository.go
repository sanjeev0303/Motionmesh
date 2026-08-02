package cdn

import (
	"context"

	"github.com/motionmesh/server/shared/models"
)

type Repository interface {
	CreateDomain(ctx context.Context, domain *models.CDNDomain) error
	GetDomain(ctx context.Context, id string) (*models.CDNDomain, error)
	ListDomains(ctx context.Context, accountID string) ([]models.CDNDomain, error)
	DeleteDomain(ctx context.Context, id string) error
	UpdateDomainStatus(ctx context.Context, id string, hostnameStatus, sslStatus string, verificationErrors *string) error
}
