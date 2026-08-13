-- Migration: 021_outbox_attempts
-- Adds columns needed for reliable outbox processing and locking

ALTER TABLE outbox_events 
    ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS claimed_until TIMESTAMPTZ;

-- Index to optimize the SKIP LOCKED query in the outbox relay
CREATE INDEX IF NOT EXISTS outbox_events_unpublished_idx 
    ON outbox_events (created_at ASC) 
    WHERE published_at IS NULL;
