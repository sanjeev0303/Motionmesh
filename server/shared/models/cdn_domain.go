package models

import "time"

type CDNDomain struct {
	ID                   string     `json:"id" db:"id"`
	AccountID            string     `json:"account_id" db:"account_id"`
	Hostname             string     `json:"hostname" db:"hostname"`
	CloudflareHostnameID *string    `json:"cloudflare_hostname_id,omitempty" db:"cloudflare_hostname_id"`
	HostnameStatus       string     `json:"hostname_status" db:"hostname_status"`
	SSLStatus            string     `json:"ssl_status" db:"ssl_status"`
	VerificationErrors   *string    `json:"verification_errors,omitempty" db:"verification_errors"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
}
