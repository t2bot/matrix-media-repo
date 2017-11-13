package schema

import "database/sql"

const thumbnailSchema = `
CREATE TABLE IF NOT EXISTS thumbnails (
	origin TEXT NOT NULL,
	media_id TEXT NOT NULL,
	width INT NOT NULL,
	height INT NOT NULL,
	method TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size_bytes BIGINT NOT NULL,
	location TEXT NOT NULL,
	creation_ts BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS thumbnails_index ON thumbnails (media_id, origin, width, height, method);
`

func PrepareThumbnails(db *sql.DB) (err error) {
	_, err = db.Exec(thumbnailSchema)
	if err != nil {
		return err
	}

	return
}