CREATE INDEX IF NOT EXISTS idx_sha256_hash_media ON media (sha256_hash);
CREATE INDEX IF NOT EXISTS idx_sha256_hash_thumbnails ON thumbnails (sha256_hash);
