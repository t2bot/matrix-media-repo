UPDATE background_tasks SET end_ts = 0 WHERE end_ts IS NULL;
ALTER TABLE background_tasks ALTER COLUMN end_ts SET NOT NULL;