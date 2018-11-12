ALTER TABLE media DROP COLUMN datastore_id;
ALTER TABLE thumbnails DROP COLUMN datastore_id;
DROP INDEX datastores_index;
DROP TABLE datastores;
