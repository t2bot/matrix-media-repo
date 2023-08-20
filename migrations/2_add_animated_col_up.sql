ALTER TABLE thumbnails ADD COLUMN animated BOOL NOT NULL DEFAULT FALSE;
DROP INDEX IF EXISTS thumbnails_index;
CREATE UNIQUE INDEX IF NOT EXISTS thumbnails_index ON thumbnails (media_id, origin, width, height, method, animated);
