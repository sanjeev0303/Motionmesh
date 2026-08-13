-- Migration: 023_outbox_billing_transcode_updates
-- Adds necessary columns to outbox_events to support exponential backoff and reliable processing

ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_error TEXT;

-- Index to optimize finding pending/failed events that are due for retry
CREATE INDEX IF NOT EXISTS outbox_events_status_next_attempt_idx
    ON outbox_events (status, next_attempt_at ASC)
    WHERE status IN ('pending', 'failed');
