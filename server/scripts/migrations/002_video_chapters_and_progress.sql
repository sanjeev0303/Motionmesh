-- Migration: 002_video_chapters_and_progress
-- Applies the changes for video processing pipeline:
--   - chapters: store AI-generated video chapters
--   - videos: add object keys for generated thumbnails, sprites, and previews
--   - transcode_jobs: add progress tracking

CREATE TABLE IF NOT EXISTS chapters (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  video_id UUID NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  start_time_seconds REAL NOT NULL,
  title TEXT NOT NULL,
  position INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE videos ADD COLUMN IF NOT EXISTS thumbnail_key TEXT;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS sprite_key TEXT;
ALTER TABLE videos ADD COLUMN IF NOT EXISTS preview_key TEXT;

ALTER TABLE transcode_jobs ADD COLUMN IF NOT EXISTS progress_percent INT NOT NULL DEFAULT 0;
