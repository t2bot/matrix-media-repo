CREATE TABLE IF NOT EXISTS expiring_media (
    origin TEXT NOT NULL,
    media_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    expires_ts BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_expiring_media ON expiring_media (media_id, origin);
CREATE INDEX IF NOT EXISTS idx_expiring_media_user_id ON expiring_media (user_id);
CREATE INDEX IF NOT EXISTS idx_expiring_media_expires_ts ON expiring_media (expires_ts);