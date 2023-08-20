ALTER TABLE media DROP COLUMN datastore_id;
ALTER TABLE thumbnails DROP COLUMN datastore_id;
DROP INDEX IF EXISTS datastores_index;
DROP TABLE IF EXISTS datastores;
