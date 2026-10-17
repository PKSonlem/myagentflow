-- name: UpsertRun :one
-- Находит Run по session_id или создаёт его. Возвращает id.
-- ON CONFLICT DO UPDATE берёт блокировку строки до конца транзакции: на ней держится
-- последовательная нумерация seq (параллельные запросы одного Run ждут друг друга).
-- Любое событие переводит Run обратно в running (resume не шлёт SessionStart по http, см. docs/notes/hooks-reality.md);
-- SessionEnd затем закрывает его через FinishRun в той же транзакции.
INSERT INTO runs (id, session_id, cwd, status, started_at)
VALUES (@id, @session_id, @cwd, 'running', @started_at)
ON CONFLICT (session_id) DO UPDATE SET
    status      = 'running',
    finished_at = NULL,
    cwd         = CASE WHEN runs.cwd = '' THEN EXCLUDED.cwd ELSE runs.cwd END
RETURNING id;

-- name: FinishRun :exec
UPDATE runs
SET status = 'finished', finished_at = @finished_at
WHERE id = @id;

-- name: SetRunTaskText :exec
-- Только первый UserPromptSubmit: если task_text уже есть, строка не меняется.
UPDATE runs
SET task_text = @task_text
WHERE id = @id AND task_text IS NULL;
