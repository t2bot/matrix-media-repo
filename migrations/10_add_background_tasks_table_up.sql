CREATE TABLE IF NOT EXISTS background_tasks (
    id SERIAL PRIMARY KEY,
    task TEXT NOT NULL,
    params JSON NOT NULL,
    start_ts BIGINT NOT NULL,
    end_ts BIGINT NULL
);
