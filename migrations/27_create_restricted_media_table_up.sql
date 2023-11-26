CREATE TABLE IF NOT EXISTS restricted_media (origin TEXT NOT NULL, media_id TEXT NOT NULL, condition_type TEXT NOT NULL, condition_value TEXT NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS idx_restricted_media ON restricted_media (origin, media_id);
