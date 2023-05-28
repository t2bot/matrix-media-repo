CREATE TABLE IF NOT EXISTS media_id_hold (
    origin TEXT NOT NULL,
    media_id TEXT NOT NULL,
    reason TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_id_hold ON media_id_hold (media_id, origin);