-- +goose Up
CREATE TABLE IF NOT EXISTS events (
    id uuid NOT NULL,
    run_id uuid NOT NULL REFERENCES runs(id),
    seq BIGINT NOT NULL,
    hook_event_name TEXT NOT NULL,
    tool_name TEXT NULL,
    tool_use_id TEXT NULL,
    ts TIMESTAMPTZ NOT NULL,
    payload JSONB NOT NULL,
    PRIMARY KEY (id, ts)
);

-- порядок внутри запуска; закрывает и чтение по (run_id, seq)
CREATE UNIQUE INDEX events_run_id_seq_idx ON events (run_id, seq);

-- идемпотентность: только события с tool_use_id
CREATE UNIQUE INDEX events_idempotency_idx
    ON events (run_id, hook_event_name, tool_use_id)
    WHERE tool_use_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS events;
