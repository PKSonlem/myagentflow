# Хуки Claude Code: что приходит на самом деле (T2)

Источник: живые сессии 2026-10-17, http-хуки на `127.0.0.1:8980`, данные из таблиц `runs` и `events` (15+ событий, 1 сессия, запускалась и возобновлялась через `backend/scripts/claude-observed.sh`).

## Вердикты по пунктам тикета
| Вопрос | Ответ |
|---|---|
| Приходит ли текст промпта (`task_text`) | Да, поле `prompt` в `UserPromptSubmit`, `runs.task_text` заполняется |
| `Stop`: после каждого ответа или в конце | После каждого ответа |
| Поддерживает ли `SessionStart` http | **Нет: не пришёл ни при свежем старте, ни при `--resume`** (первая сессия началась с `UserPromptSubmit`) |
| Что приходит при `--resume` | Тот же `session_id`, ни `SessionStart`, ни другого маркера возобновления; события просто продолжаются |
| `cwd` и `tool_use_id` во всех событиях | `cwd` и `session_id` во всех 6 типах. `tool_use_id` только у `PreToolUse`, `PostToolUse`, `PostToolUseFailure` (у `UserPromptSubmit`, `Stop`, `SessionEnd` его нет) |
| Повторы http-хука | Не проверено: уникальный индекс `events_idempotency_idx` (`ON CONFLICT DO NOTHING`) молча скрывает дубли, по БД их не отличить |
| `\u0000` в payload | Бывает, сервер отвечает 400, событие теряется (см. ниже) |
| Остановка сервера | Claude Code не зависает, события за время простоя теряются |

## Ключи payload по событиям
- `UserPromptSubmit`: `cwd, prompt, prompt_id, session_id, scratchpad_dir, hook_event_name, permission_mode, transcript_path`
- `PreToolUse`: `cwd, effort, prompt_id, tool_name, session_id, tool_input, tool_use_id, scratchpad_dir, hook_event_name, permission_mode, transcript_path`
- `PostToolUse`: то же, плюс `duration_ms, tool_response`
- `PostToolUseFailure`: как `PreToolUse`, плюс `error, duration_ms, is_interrupt`
- `Stop`: `cwd, effort, prompt_id, session_id, session_crons, scratchpad_dir, hook_event_name, permission_mode, transcript_path, background_tasks, stop_hook_active, last_assistant_message`
- `SessionEnd`: `cwd, reason, prompt_id, session_id, scratchpad_dir, hook_event_name, transcript_path` (`reason: prompt_input_exit` при `/exit`)

## Реальные примеры payload (UUID сессии и пути без секретов)
`UserPromptSubmit`:
```json
{
  "cwd": "/Users/kirill/Golang/Пет проекты/myagentflow",
  "prompt": "Привет",
  "prompt_id": "7070625b-12ab-4b44-9bda-39d4a7d6e7aa",
  "session_id": "3b00eae5-8843-4a0c-a44c-3e5e15a41a9f",
  "hook_event_name": "UserPromptSubmit",
  "permission_mode": "auto",
  "transcript_path": "/Users/kirill/.claude/projects/<проект>/3b00eae5-8843-4a0c-a44c-3e5e15a41a9f.jsonl",
  "scratchpad_dir": "/private/tmp/claude-501/<проект>/<session_id>/scratchpad"
}
```
`PostToolUseFailure` (Read несуществующего файла):
```json
{
  "cwd": "/Users/kirill/Golang/Пет проекты/myagentflow",
  "error": "File does not exist. Note: your current working directory is /Users/kirill/Golang/Пет проекты/myagentflow.",
  "effort": {"level": "medium"},
  "tool_name": "Read",
  "tool_input": {"file_path": "/nonexistent-file"},
  "tool_use_id": "toolu_016XHiJhVp9az9jkhnVrvAqq",
  "duration_ms": 11,
  "is_interrupt": false,
  "hook_event_name": "PostToolUseFailure",
  "permission_mode": "auto",
  "session_id": "3b00eae5-8843-4a0c-a44c-3e5e15a41a9f",
  "prompt_id": "fc50ad65-fa2a-4999-a305-06ff8bfdab50"
}
```
`SessionEnd`:
```json
{
  "cwd": "/Users/kirill/Golang/Пет проекты/myagentflow",
  "reason": "prompt_input_exit",
  "hook_event_name": "SessionEnd",
  "session_id": "3b00eae5-8843-4a0c-a44c-3e5e15a41a9f",
  "prompt_id": "5ce5fb70-3057-400d-b77f-fc7ee08b38b0"
}
```
(`transcript_path` и `scratchpad_dir` в примерах сокращены.)

## --resume
- `SessionStart` не приходит, события возобновлённой сессии (тот же `session_id`) ложатся в уже `finished` Run.
- Без правки это показало бы идущую сессию завершённой. Правка в T2: любое событие переводит Run в `running` и сбрасывает `finished_at` (`UpsertRun`, тест, `feature.md`, `GLOSSARY.md`). Проверено вживую: после `SessionEnd` следующий `UserPromptSubmit` вернул Run в `running`.
- Известный компромисс: запоздавшее событие после `SessionEnd` тоже переоткроет Run. Порядок прихода может отличаться от порядка вызовов (см. ниже). В тестах это поведение зафиксировано.
- Пропуск событий в первом возобновлении (промпты 11:56-11:58 UTC) вероятно связан с запуском без `MYAGENTFLOW_TOKEN`; это догадка, не проверялось. После запуска через скрипт события пошли.

## PostToolUseFailure
- Приходит и для Bash (`Exit code 1` + stderr в поле `error`), и для Read (`File does not exist...`). Обычного `PostToolUse` для упавших вызовов нет.
- Порядок прихода (`seq`) не совпал с порядком вызова: `PostToolUseFailure` Read (seq 13) пришёл раньше Bash (seq 14), хотя Bash был запущен первым (параллельные вызовы).

## Остановка сервера
- Сервер выключен 12:06:56-12:07:44 UTC. Промпт «ТЫ не завис?» (12:07:10) получил ответ за 8 секунд, хуки этого промпта в БД не попали, повторной доставки нет. Claude Code не завис. Проверялся промпт без вызовов инструментов; `PreToolUse`/`PostToolUse` при выключенном сервере отдельно не проверялись (таймаут хука один и тот же, 2 секунды).

## \u0000 в payload
- `curl` с `\u0000` в теле: 400 `NUL character (\u0000) is not supported`, Run и событие не создаются.
- Реально бывает: `printf 'a\0b\n'` через Bash вернул `'a\x00b'`. В БД есть `PreToolUse` (seq 22), `PostToolUse` для этого вызова нет, `Stop` (seq 23) пришёл. Claude Code NUL в `tool_response` не вырезает.
- Вывод: вызов с бинарным выводом теряет `PostToolUse`. Нужно вырезать или заменять NUL до записи, а не отвечать 400. Это отдельная правка вне T2.

## Не проверено
- Повторы http-хука (см. таблицу выше).
- `PreToolUse`/`PostToolUse` при выключенном сервере.
