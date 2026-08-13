-- Migration: 026_transcode_jobs_uniqueness
-- Ensures strict one-job-per-video constraint at the database layer

-- First remove any duplicate transcode jobs, keeping only the oldest one per video
DELETE FROM transcode_jobs WHERE id NOT IN (SELECT MIN(id) FROM transcode_jobs GROUP BY video_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'idx_transcode_jobs_video_id'
    ) THEN
        ALTER TABLE transcode_jobs ADD CONSTRAINT idx_transcode_jobs_video_id UNIQUE (video_id);
    END IF;
END $$;
