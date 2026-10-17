# T1: Ingest API + Postgres

**Цель:** сервер принимает нативный payload http-хука Claude Code и идемпотентно пишет его в Postgres.

## Кто что пишет
- Агент (рутина): `docker-compose.yml` с Postgres, `go mod init`, скелет `backend/cmd/server`, Makefile, sqlc/goose конфиги, `oapi-codegen` и `make generate`.
- Владелец (ядро): миграции, SQL-запросы, адаптер `claudecode`, реализация сгенерированного `StrictServerInterface` (метод `IngestClaudeHook`), middleware токена, тесты.

## Контекст для чтения
- `docs/ROADMAP.md` (решения)
- `docs/features/F01-observe/feature.md` (модель данных)
- Документация Claude Code по hooks: общие поля входа (`session_id`, `transcript_path`, `cwd`, `hook_event_name`, `tool_name`, `tool_input`, `tool_use_id`)

## Что сделать
1. Миграции goose для `runs` и `events` (см. feature.md).
2. Эндпоинт `POST /hooks/claude`:
   - проверка токена из заголовка `Authorization: Bearer` (`crypto/subtle`), иначе 401;
   - адаптер `claudecode`: JSON хука → внутренняя структура события;
   - лимит тела 10 МиБ (`http.MaxBytesReader`), сверх лимита `413`;
   - upsert `runs` по `session_id`: любое событие новой сессии создаёт Run (`started_at` = время получения первого события), `SessionStart` заново открывает существующий (`running`, `finished_at = NULL`), закрывает только `SessionEnd`; `Stop` это обычное событие (по документации `SessionStart` может не поддерживать http-хуки, см. открытые вопросы в T0);
   - `task_text` заполняется из первого `UserPromptSubmit`;
   - вставка события с `ON CONFLICT DO NOTHING` по частичному ключу идемпотентности (только события с `tool_use_id`), остальные пишутся всегда; `seq` назначается в БД внутри транзакции;
   - ответ быстро, `200` с пустым телом (хук не ждёт).
3. Неизвестный `hook_event_name` сохраняется как есть, без ошибки.

## Критерии приёмки
- Повторная отправка того же `PreToolUse`/`PostToolUse` не создаёт второго события.
- Два `Stop` в одной сессии дают два события.
- `SessionEnd` закрывает Run, повторный `SessionStart` открывает его снова.
- Два параллельных запроса на один `run` дают разные `seq` без дублей.
- Тесты на реальном Postgres (testcontainers): идемпотентность и параллельность, тест сначала.
- Без токена 401.

## Вне тикета
Чтение данных, фронт, настройка хуков, маскирование.
