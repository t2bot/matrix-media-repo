ALTER TABLE thumbnails ALTER COLUMN sha256_hash SET NULL;
UPDATE thumbnails SET sha256_hash = NULL WHERE sha256_hash = '';