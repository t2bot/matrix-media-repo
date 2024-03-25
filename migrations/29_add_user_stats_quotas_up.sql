ALTER TABLE user_stats ADD COLUMN quota_max_bytes BIGINT NOT NULL DEFAULT '-1';
ALTER TABLE user_stats ADD COLUMN quota_max_pending BIGINT NOT NULL DEFAULT '-1';
ALTER TABLE user_stats ADD COLUMN quota_max_files BIGINT NOT NULL DEFAULT '-1';
