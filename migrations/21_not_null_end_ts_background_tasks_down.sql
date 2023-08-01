ALTER TABLE background_tasks ALTER COLUMN end_ts SET NULL;
UPDATE background_tasks SET end_ts = NULL WHERE end_ts = 0;