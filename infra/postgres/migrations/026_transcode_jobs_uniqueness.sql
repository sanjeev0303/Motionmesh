-- Migration: 026_transcode_jobs_uniqueness
-- Ensures strict one-job-per-video constraint at the database layer

ALTER TABLE transcode_jobs ADD CONSTRAINT idx_transcode_jobs_video_id UNIQUE (video_id);
