DROP INDEX IF EXISTS thumbnails_index;
CREATE UNIQUE INDEX IF NOT EXISTS thumbnails_index ON thumbnails (media_id, origin, width, height, method);
ALTER TABLE thumbnails DROP COLUMN animated;
