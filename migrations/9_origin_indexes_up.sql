CREATE INDEX IF NOT EXISTS idx_origin_media ON media(origin);
CREATE INDEX IF NOT EXISTS idx_origin_thumbnails ON thumbnails(origin);
CREATE INDEX IF NOT EXISTS idx_origin_user_id_media ON media(origin, user_id);
