package schema

import "database/sql"

const mediaSchema = `
CREATE TABLE IF NOT EXISTS media (
	origin TEXT NOT NULL,
	media_id TEXT NOT NULL,
	upload_name TEXT NOT NULL,
	content_type TEXT NOT NULL,
	user_id TEXT NOT NULL,
	sha256_hash TEXT NOT NULL,
	size_bytes BIGINT NOT NULL,
	location TEXT NOT NULL,
	creation_ts BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS media_index ON media (media_id, origin);
`

func PrepareMedia(db *sql.DB) (err error) {
	_, err = db.Exec(mediaSchema)
	if err != nil {
		return err
	}

	return
}