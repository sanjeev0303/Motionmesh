-- Migration: 001_reconcile_architecture
-- Applies the changes from the architecture reconciliation:
--   - accounts: drop password_hash, add Clerk identity + plan/status
--   - usage_events: source-of-truth usage ledger
--   - watermark_metadata: branding config per account (pro tier)
--   - analytics_daily: analytics aggregation with atomic upsert support

-- ─── accounts ────────────────────────────────────────────────────────────────
-- Clerk owns credentials now; password_hash is no longer needed.
ALTER TABLE accounts DROP COLUMN IF EXISTS password_hash;

ALTER TABLE accounts
  ADD COLUMN IF NOT EXISTS clerk_user_id TEXT UNIQUE,
  ADD COLUMN IF NOT EXISTS clerk_org_id  TEXT UNIQUE,
  ADD COLUMN IF NOT EXISTS stripe_customer_id TEXT UNIQUE,
  ADD COLUMN IF NOT EXISTS plan          TEXT NOT NULL DEFAULT 'free',
  ADD COLUMN IF NOT EXISTS status        TEXT NOT NULL DEFAULT 'active';

CREATE INDEX IF NOT EXISTS accounts_clerk_user_id_idx ON accounts (clerk_user_id) WHERE clerk_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS accounts_clerk_org_id_idx  ON accounts (clerk_org_id)  WHERE clerk_org_id  IS NOT NULL;

-- ─── api_keys ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS api_keys (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id   UUID        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  name         TEXT        NOT NULL,
  prefix       TEXT        NOT NULL UNIQUE,
  hash         TEXT        NOT NULL,
  scopes       TEXT[]      NOT NULL DEFAULT '{}',
  last_used_at TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ,
  revoked_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS api_keys_account_id_idx ON api_keys (account_id);

-- ─── usage_events ─────────────────────────────────────────────────────────────
-- Source of truth for metered billing. Stripe Meter Events are projections of this table.
CREATE TABLE IF NOT EXISTS usage_events (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id  UUID        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  event_type  TEXT        NOT NULL,   -- 'storage_bytes' | 'transcode_minutes' | 'bandwidth_bytes'
  quantity    BIGINT      NOT NULL DEFAULT 1,
  metadata    JSONB,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS usage_events_account_id_idx ON usage_events (account_id);
CREATE INDEX IF NOT EXISTS usage_events_created_at_idx ON usage_events (created_at DESC);

-- ─── watermark_metadata ───────────────────────────────────────────────────────
-- Pro-tier only. Transcode worker reads the active row when building ffmpeg command.
CREATE TABLE IF NOT EXISTS watermark_metadata (
  id               UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id       UUID    NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  asset_object_key TEXT    NOT NULL,
  position         TEXT    NOT NULL DEFAULT 'bottom-right',
  opacity          REAL    NOT NULL DEFAULT 0.8,
  is_active        BOOLEAN NOT NULL DEFAULT true,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS watermark_metadata_account_active_idx
  ON watermark_metadata (account_id)
  WHERE is_active = true;

-- ─── analytics_daily ─────────────────────────────────────────────────────────
-- Atomic upsert target: INSERT ... ON CONFLICT DO UPDATE SET views = views + 1
-- No application-level read-modify-write; the primary key is the conflict target.
CREATE TABLE IF NOT EXISTS analytics_daily (
  account_id  UUID   NOT NULL,
  video_id    UUID   NOT NULL,
  day         DATE   NOT NULL,
  country     TEXT   NOT NULL DEFAULT '',
  views       BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (account_id, video_id, day, country)
);
