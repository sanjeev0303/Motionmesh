-- Add unique constraint to renditions to support deduplication
ALTER TABLE renditions ADD CONSTRAINT unique_video_resolution UNIQUE (video_id, resolution);
