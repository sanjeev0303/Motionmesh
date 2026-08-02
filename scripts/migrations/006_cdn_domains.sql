-- scripts/migrations/006_cdn_domains.sql
-- Create cdn_domains table

CREATE TABLE IF NOT EXISTS cdn_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    hostname VARCHAR(255) NOT NULL UNIQUE,
    cloudflare_hostname_id VARCHAR(255),
    hostname_status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, active, active_redeploying, moved, deleted, etc.
    ssl_status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, active, validation_timed_out, etc.
    verification_errors JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cdn_domains_account_id ON cdn_domains(account_id);
CREATE INDEX idx_cdn_domains_hostname ON cdn_domains(hostname);
