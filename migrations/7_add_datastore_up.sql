CREATE TABLE IF NOT EXISTS datastores (
  datastore_id TEXT NOT NULL,
  ds_type TEXT NOT NULL,
  uri TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS datastores_index ON datastores (datastore_id);

ALTER TABLE media ADD COLUMN datastore_id TEXT NOT NULL DEFAULT '';
ALTER TABLE thumbnails ADD COLUMN datastore_id TEXT NOT NULL DEFAULT '';

