# F01: Observe

**Результат:** запускаю обычную сессию Claude Code и вижу её в браузере: список запусков и таймлайн шагов.

## Подход: сначала контракт, потом код

Я намерен сначала описать OpenAPI-спеку (с помощью агента: он задаёт вопросы по контракту, указывает на пропущенные случаи и ошибки в схеме, но не пишет спеку за меня), и только после этого писать реализацию. Спека это источник истины для бэкенда и фронта: из неё генерируются серверный интерфейс (`oapi-codegen`) и типы фронта (`openapi-typescript`), см. [ADR-0001](../../adr/0001-openapi-codegen-contract-first.md). Любое изменение API начинается с правки спеки и `make generate`.

## Тикеты (по порядку)

0. [T0 OpenAPI-контракт](T0-openapi.md)
1. [T1 Ingest API + Postgres](T1-ingest.md)
2. [T2 Хуки Claude Code](T2-hooks.md)
3. [T3 Read API](T3-read-api.md)
4. [T4 React: список и страница запуска](T4-web.md)

Порядок важен: T0 фиксирует контракт, T2 сразу даёт живые данные в БД, T3 и T4 строятся на них.

## Модель данных (минимум)

```text
runs(id uuid v7, source, session_id UNIQUE, cwd, task_text NULL, status, started_at, finished_at NULL)
  -- status: running | finished (failed появится, когда будет правило)
events(id, run_id, seq, hook_event_name, tool_name, tool_use_id NULL, ts, payload jsonb)
  -- частичный UNIQUE(run_id, hook_event_name, tool_use_id) WHERE tool_use_id IS NOT NULL
  -- события без tool_use_id (SessionStart, Stop, SessionEnd...) пишутся всегда
  -- PRIMARY KEY должен включать ts (задел под партиционирование)
```

Run = одна сессия (`session_id`). Создаёт его любое первое событие сессии (`started_at` = время получения), закрывает только `SessionEnd`; `Stop` срабатывает после каждого ответа и остаётся обычным событием. Повторный `SessionStart` с тем же `session_id` (resume) снова переводит Run в `running` и сбрасывает `finished_at`.

`events.ts` = время получения сервером (в payload хука своей метки времени нет).

`task_text` = текст первого `UserPromptSubmit` в Run; `NULL`, пока промпта не было. Термины: [GLOSSARY.md](../../../GLOSSARY.md).

## Вне F01

Подтверждения, Telegram, политики, детектор, токены и стоимость, SSE, партиционирование, воркер `abandoned`, логин в UI.

## Готово, когда

В живой сессии Claude Code я выполняю пару команд, обновляю страницу и вижу запуск с шагами в правильном порядке.
