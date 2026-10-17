-- +goose Up
CREATE TABLE IF NOT EXISTS runs (
    id uuid PRIMARY KEY,
    session_id TEXT UNIQUE NOT NULL,
    cwd TEXT NOT NULL DEFAULT '',
    task_text TEXT NULL,
    status TEXT NOT NULL CHECK(status IN ('running','finished')),
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NULL
);

CREATE INDEX runs_started_at_id_idx ON runs(started_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS runs;
