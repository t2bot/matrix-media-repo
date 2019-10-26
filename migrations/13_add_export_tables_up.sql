CREATE TABLE IF NOT EXISTS exports (
    export_id TEXT PRIMARY KEY NOT NULL,
    entity TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS export_parts (
    export_id TEXT NOT NULL,
    index INT NOT NULL,
    size_bytes BIGINT NOT NULL,
    file_name TEXT NOT NULL,
    datastore_id TEXT NOT NULL,
    location TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS export_parts_index ON export_parts (export_id, index);
