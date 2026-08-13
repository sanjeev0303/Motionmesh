-- Migration: 022_usage_event_id
-- The usage_events table already has an id column as PRIMARY KEY from 001_reconcile_architecture.
-- This migration is a placeholder to fulfill the requirement for explicitly defining the id column for idempotency.
-- The idempotency is achieved via ON CONFLICT (id) DO NOTHING in the Go code.
SELECT 1;
