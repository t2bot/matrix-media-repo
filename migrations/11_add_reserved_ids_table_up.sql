CREATE TABLE IF NOT EXISTS reserved_media (
	origin TEXT NOT NULL,
	media_id TEXT NOT NULL,
	reason TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS reserved_media_index ON reserved_media (media_id, origin);
