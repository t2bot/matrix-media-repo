CREATE TABLE IF NOT EXISTS media_attributes (
	origin TEXT NOT NULL,
	media_id TEXT NOT NULL,
	purpose TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_attributes ON media_attributes (media_id, origin);
CREATE INDEX IF NOT EXISTS idx_media_attributes_purpose on media_attributes (purpose);
