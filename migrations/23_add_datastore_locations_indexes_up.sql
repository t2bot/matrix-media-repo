CREATE INDEX IF NOT EXISTS idx_datastore_id_location_thumbnails ON thumbnails(datastore_id, location);
CREATE INDEX IF NOT EXISTS idx_datastore_id_location_media ON media(datastore_id, location);
