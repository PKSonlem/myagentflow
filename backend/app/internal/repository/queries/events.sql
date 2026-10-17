-- name: InsertEvent :execrows
-- seq = следующий номер внутри Run. Безопасно только в транзакции после UpsertRun,
-- который уже заблокировал строку Run.
-- Возвращает число вставленных строк: 0 значит дубль по ключу идемпотентности.
INSERT INTO events (id, run_id, seq, hook_event_name, tool_name, tool_use_id, ts, payload)
SELECT @id::uuid, @run_id::uuid, COALESCE(MAX(seq), 0) + 1, @hook_event_name::text,
       sqlc.narg(tool_name)::text, sqlc.narg(tool_use_id)::text, @ts::timestamptz, @payload::jsonb
FROM events
WHERE run_id = @run_id::uuid
ON CONFLICT (run_id, hook_event_name, tool_use_id) WHERE tool_use_id IS NOT NULL
DO NOTHING;

-- name: ListRunEvents :many
-- События запуска строго после after_seq по возрастанию seq. Идёт по events_run_id_seq_idx.
SELECT * FROM events
WHERE run_id = @run_id::uuid AND seq > @after_seq::bigint
ORDER BY seq
LIMIT @page_size;
