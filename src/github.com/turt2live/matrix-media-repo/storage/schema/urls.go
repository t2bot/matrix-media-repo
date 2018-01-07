package schema

import "database/sql"

const urlSchema = `
CREATE TABLE IF NOT EXISTS url_previews (
	url TEXT NOT NULL,
	error_code TEXT NOT NULL,
	bucket_ts BIGINT NOT NULL,
	site_url TEXT NOT NULL,
	site_name TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	description TEXT NOT NULL,
	title TEXT NOT NULL,
	image_mxc TEXT NOT NULL,
	image_type TEXT NOT NULL,
	image_size BIGINT NOT NULL,
	image_width INT NOT NULL,
	image_height INT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS url_previews_index ON url_previews (url, error_code, bucket_ts);
`

func PrepareUrls(db *sql.DB) (err error) {
	_, err = db.Exec(urlSchema)
	if err != nil {
		return err
	}

	return
}
