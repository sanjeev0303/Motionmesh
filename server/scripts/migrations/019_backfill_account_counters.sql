-- Migration: 019_backfill_account_counters
-- Backfills the account usage counters from existing video data

UPDATE accounts a
SET total_videos = COALESCE((SELECT COUNT(*) FROM videos v WHERE v.account_id = a.id), 0),
    total_storage_bytes = COALESCE((SELECT SUM(size_bytes) FROM videos v WHERE v.account_id = a.id), 0);
