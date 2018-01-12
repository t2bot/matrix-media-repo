-- MEDIA
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

-- THUMBNAILS
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

-- URL PREVIEWS
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
