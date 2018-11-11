CREATE TABLE IF NOT EXISTS last_access (
  sha256_hash TEXT NOT NULL,
  last_access_ts BIGINT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS last_access_index ON last_access (sha256_hash);