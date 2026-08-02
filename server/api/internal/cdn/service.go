package cdn

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/motionmesh/server/shared/config"
	"github.com/motionmesh/server/shared/models"
)

type Stats struct {
	OutboundBytes int64 `json:"outbound_bytes"`
	CostCents     int64 `json:"cost_cents"`
}

type Service interface {
	AddCustomDomain(ctx context.Context, accountID, hostname string) (*models.CDNDomain, error)
	RefreshDomainStatus(ctx context.Context, id string) (*models.CDNDomain, error)
	DeleteDomain(ctx context.Context, id string) error
	ListDomains(ctx context.Context, accountID string) ([]models.CDNDomain, error)
	GetStats(ctx context.Context, accountID string) (*Stats, error)
	GenerateSignedURL(path string, ttlSeconds int64) (string, error)
}

type cdnService struct {
	repo   Repository
	config *config.Config
}

func NewCDNService(repo Repository, cfg *config.Config) Service {
	return &cdnService{
		repo:   repo,
		config: cfg,
	}
}

func (s *cdnService) isCloudflareConfigured() bool {
	return s.config.CloudflareAPIToken != "" && s.config.CloudflareZoneID != ""
}

func (s *cdnService) AddCustomDomain(ctx context.Context, accountID, hostname string) (*models.CDNDomain, error) {
	if !s.isCloudflareConfigured() {
		return nil, errors.New("Cloudflare not configured")
	}

	// 1. Call Cloudflare Custom Hostnames API
	cfURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/custom_hostnames", s.config.CloudflareZoneID)
	payload := map[string]interface{}{
		"hostname": hostname,
		"ssl": map[string]string{
			"method": "txt",
			"type":   "dv",
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, cfURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Cloudflare API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Cloudflare API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var cfResp struct {
		Success bool `json:"success"`
		Result  struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			SSL    struct {
				Status string `json:"status"`
			} `json:"ssl"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("failed to decode Cloudflare response: %w", err)
	}

	if !cfResp.Success {
		return nil, errors.New("Cloudflare returned failure")
	}

	// 2. Save to DB
	domain := &models.CDNDomain{
		AccountID:            accountID,
		Hostname:             hostname,
		CloudflareHostnameID: &cfResp.Result.ID,
		HostnameStatus:       cfResp.Result.Status,
		SSLStatus:            cfResp.Result.SSL.Status,
	}

	if err := s.repo.CreateDomain(ctx, domain); err != nil {
		return nil, fmt.Errorf("failed to create domain in DB: %w", err)
	}

	return domain, nil
}

func (s *cdnService) RefreshDomainStatus(ctx context.Context, id string) (*models.CDNDomain, error) {
	domain, err := s.repo.GetDomain(ctx, id)
	if err != nil {
		return nil, err
	}

	if !s.isCloudflareConfigured() || domain.CloudflareHostnameID == nil {
		return domain, nil
	}

	cfURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/custom_hostnames/%s", s.config.CloudflareZoneID, *domain.CloudflareHostnameID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfURL, nil)
	req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var cfResp struct {
			Success bool `json:"success"`
			Result  struct {
				Status string `json:"status"`
				SSL    struct {
					Status            string `json:"status"`
					ValidationErrors  []struct {
						Message string `json:"message"`
					} `json:"validation_errors"`
				} `json:"ssl"`
			} `json:"result"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&cfResp); err == nil && cfResp.Success {
			var errorsStr *string
			if len(cfResp.Result.SSL.ValidationErrors) > 0 {
				errs, _ := json.Marshal(cfResp.Result.SSL.ValidationErrors)
				str := string(errs)
				errorsStr = &str
			}
			
			err = s.repo.UpdateDomainStatus(ctx, id, cfResp.Result.Status, cfResp.Result.SSL.Status, errorsStr)
			if err != nil {
				return nil, err
			}
			domain.HostnameStatus = cfResp.Result.Status
			domain.SSLStatus = cfResp.Result.SSL.Status
			domain.VerificationErrors = errorsStr
		}
	}

	return domain, nil
}

func (s *cdnService) DeleteDomain(ctx context.Context, id string) error {
	domain, err := s.repo.GetDomain(ctx, id)
	if err != nil {
		return err
	}

	if s.isCloudflareConfigured() && domain.CloudflareHostnameID != nil {
		cfURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/custom_hostnames/%s", s.config.CloudflareZoneID, *domain.CloudflareHostnameID)
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, cfURL, nil)
		req.Header.Set("Authorization", "Bearer "+s.config.CloudflareAPIToken)
		
		client := &http.Client{Timeout: 10 * time.Second}
		client.Do(req) // Fire and forget or check error
	}

	return s.repo.DeleteDomain(ctx, id)
}

func (s *cdnService) ListDomains(ctx context.Context, accountID string) ([]models.CDNDomain, error) {
	return s.repo.ListDomains(ctx, accountID)
}

func (s *cdnService) GetStats(ctx context.Context, accountID string) (*Stats, error) {
	// In a real application, this would query usage_events for egress_gb
	// Since this is a placeholder implementation without access to the actual usage_events repo,
	// we will just return 0 for now. The dashboard uses mock data anyway.
	return &Stats{
		OutboundBytes: 0,
		CostCents:     0,
	}, nil
}

func (s *cdnService) GenerateSignedURL(path string, ttlSeconds int64) (string, error) {
	if s.config.CDNSigningSecret == "" {
		return "", errors.New("CDN signing secret not configured")
	}

	expiry := time.Now().Unix() + ttlSeconds
	expiryStr := fmt.Sprintf("%d", expiry)

	message := path + ":" + expiryStr
	mac := hmac.New(sha256.New, []byte(s.config.CDNSigningSecret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("exp=%s&sig=%s", expiryStr, signature), nil
}
