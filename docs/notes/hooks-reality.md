# Хуки Claude Code: что приходит на самом деле (T2)

Источник: живая сессия 2026-10-17, http-хуки на `127.0.0.1:8980`, данные из таблиц `runs` и `events`.

## Подтверждено (сессия 1: 2 промпта, 2 вызова Bash)
- Порядок и `seq` в БД: `UserPromptSubmit`, `Stop`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PreToolUse`, `PostToolUse`, `Stop`.
- `Stop` срабатывает после каждого ответа, а не только в конце сессии.
- `UserPromptSubmit` содержит текст промпта в поле `prompt`, `runs.task_text` заполняется («Привет»).
- `cwd` и `session_id` есть во всех событиях.
- `tool_use_id` есть в `PreToolUse` и `PostToolUse`; в `UserPromptSubmit` и `Stop` его нет (ожидаемо).
- `run` создался с первого же `UserPromptSubmit`, статус `running`.

## Ключи payload по событиям
- `UserPromptSubmit`: `cwd, prompt, prompt_id, session_id, scratchpad_dir, hook_event_name, permission_mode, transcript_path`
- `PreToolUse`: `cwd, effort, prompt_id, tool_name, session_id, tool_input, tool_use_id, scratchpad_dir, hook_event_name, permission_mode, transcript_path`
- `PostToolUse`: то же, плюс `duration_ms, tool_response`
- `Stop`: `cwd, effort, prompt_id, session_id, session_crons, scratchpad_dir, hook_event_name, permission_mode, transcript_path, background_tasks, stop_hook_active, last_assistant_message`

- `SessionEnd` приходит при `/exit` (`reason: prompt_input_exit`), Run переходит в `finished` с `finished_at`.

## Расхождения с ожиданиями
- `SessionStart` в первой сессии не пришёл (в БД его нет). Либо http не поддерживается для этого события, либо он сработал до подключения. Нужна отдельная проверка на свежем запуске.

## \u0000 в payload (проверено)
- `curl` с `\u0000` в теле: сервер отвечает 400 `NUL character (\u0000) is not supported`, Run и событие не создаются.
- Реально бывает: `printf 'a\0b\n'` через Bash вернул `'a\x00b'`. В БД есть `PreToolUse` (seq 22), `PostToolUse` для этого вызова нет (потерян, `Stop` seq 23 пришёл). Claude Code NUL в `tool_response` не вырезает.
- Вывод: любой вызов с бинарным выводом теряет `PostToolUse`. Нужно решение (вырезать/заменять NUL до записи, а не отвечать 400), это отдельная правка вне T2.

## Не проверено
- `PostToolUseFailure`.
- `--resume`: что приходит, остаётся ли Run `finished`.
- Повторы http-хука.
- `\u0000` в payload (сервер отвечает 400).
- Остановка сервера: не зависает ли Claude Code.

## --resume (проверено, сессия 3b00eae5 возобновлена через claude-observed.sh)
- `SessionStart` при `--resume` по http **не пришёл**. Событие в БД ни разу не появлялось.
- События возобновлённой сессии (seq 10-15, `UserPromptSubmit`, `PreToolUse` и т.д.) легли в тот же Run (тот же `session_id`), но Run остался `finished`. Риск из T1 подтверждён: UI показал бы идущую сессию завершённой.
- Вывод: переводить Run `finished` в `running` по любому событию (правка `UpsertRun` + тест + `feature.md`/`GLOSSARY.md`), `SessionStart` для этого не нужен.
- Предыдущий пропуск событий (промпты в 11:56-11:58 UTC) объясняется, скорее всего, запуском без `MYAGENTFLOW_TOKEN`: после запуска через скрипт события пошли.

## PostToolUseFailure (проверено)
- Приходит и для Bash (`Exit code 1` + stderr в поле `error`), и для Read (`File does not exist...`). Обычного `PostToolUse` для упавших вызовов нет.
- Порядок событий в БД (`seq`) не совпал с порядком вызова: `PostToolUseFailure` Read (seq 13) пришёл раньше Bash (seq 14), хотя Bash был запущен первым (параллельные вызовы).

## Остановка сервера (проверено)
- Сервер был выключен 12:06:56-12:07:44 UTC. В это окно в сессии прошёл промпт «ТЫ не завис?» (12:07:10): ответ пришёл за 8 секунд (12:07:18), хуки на это событие в БД не попали (потеряны, повторной доставки нет).
- Claude Code не завис, ответ пришёл без заметной задержки.
- Подтверждена правка UpsertRun: после `SessionEnd` (12:04:43) следующий `UserPromptSubmit` при --resume вернул Run в `running` без `SessionStart`.

## Не проверено
- Повторы http-хука: дублей `tool_use_id` в БД не наблюдалось, специально не воспроизводилось.
