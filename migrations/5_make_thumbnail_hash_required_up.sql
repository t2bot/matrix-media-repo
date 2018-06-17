UPDATE thumbnails SET sha256_hash = '' WHERE sha256_hash IS NULL;
ALTER TABLE thumbnails ALTER COLUMN sha256_hash SET NOT NULL;
