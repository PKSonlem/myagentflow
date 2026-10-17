-- name: UpsertRun :one
-- Находит Run по session_id или создаёт его. Возвращает id.
-- ON CONFLICT DO UPDATE берёт блокировку строки до конца транзакции: на ней держится
-- последовательная нумерация seq (параллельные запросы одного Run ждут друг друга).
-- reopen = true только для SessionStart: переводит Run обратно в running.
INSERT INTO runs (id, session_id, cwd, status, started_at)
VALUES (@id, @session_id, @cwd, 'running', @started_at)
ON CONFLICT (session_id) DO UPDATE SET
    status      = CASE WHEN @reopen::boolean THEN 'running' ELSE runs.status END,
    finished_at = CASE WHEN @reopen::boolean THEN NULL ELSE runs.finished_at END,
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
