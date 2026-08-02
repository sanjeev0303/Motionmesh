-- Migration: 000_init_schema
-- Foundational DDL based on Go models.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ─── accounts ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS accounts (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  email       TEXT        NOT NULL,
  password_hash TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── buckets ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS buckets (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id  UUID        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  name        TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── objects ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS objects (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  bucket_id   UUID        NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
  key         TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── videos ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS videos (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id       UUID        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
  bucket_id        TEXT        NOT NULL,
  object_key       TEXT        NOT NULL,
  title            TEXT        NOT NULL,
  status           TEXT        NOT NULL DEFAULT 'queued',
  duration         REAL        NOT NULL DEFAULT 0,
  size_bytes       BIGINT      NOT NULL DEFAULT 0,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── transcode_jobs ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS transcode_jobs (
  id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  video_id         UUID        NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  status           TEXT        NOT NULL DEFAULT 'queued',
  error_msg        TEXT,
  started_at       TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── renditions ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS renditions (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  video_id    UUID        NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  resolution  TEXT        NOT NULL,
  object_key  TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── caption_tracks ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS caption_tracks (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  video_id    UUID        NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  language    TEXT        NOT NULL,
  object_key  TEXT        NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
