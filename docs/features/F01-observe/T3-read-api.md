# T3: Read API + OpenAPI

**Цель:** отдать запуски и события в JSON с описанным контрактом.

**Статус:** выполнен

## Кто что пишет
- Агент (рутина): настройка генерации типов в `web/` (`openapi-typescript` в npm-скрипте).
- Владелец (ядро): sqlc-запросы, реализация сгенерированных методов `ListRuns`, `GetRun`, `ListRunEvents`, тесты.

## Контекст
- `feature.md` (модель данных), T1 (структуры и пакеты).

## Что сделать
1. Контракт уже описан в T0 (`backend/api/openapi.yaml`). Реализуйте строго по нему; если нужно изменить, сначала правьте спеку:
   - `GET /runs?limit&cursor` : список, новые сверху, курсорная пагинация (непрозрачный `cursor` из `started_at, id`, не OFFSET), `next_cursor` равен `null` на последней странице;
   - `GET /runs/{id}` : только запуск, `404` если нет;
   - `GET /runs/{id}/events?after_seq&limit` : события по `seq`, `next_after_seq`.
2. Хендлеры и sqlc-запросы. Токен на всех эндпоинтах.
3. Поля ответа: у Run: id, статус, cwd, task_text (nullable), started_at, finished_at (nullable), `duration_ms` (считает сервер, `null` пока `running`); у события: seq, hook_event_name, tool_name, ts, payload.
4. Индекс под выборку `events` по `(run_id, seq)`; проверить `EXPLAIN ANALYZE`.

## Критерии приёмки
- Ответы совпадают со спекой (гарантирует сгенерированный интерфейс; плюс тест на валидность спеки).
- Пагинация стабильна при добавлении новых запусков.
- Тесты на реальном Postgres.

## Вне тикета
Фильтры по агенту/проекту, SSE, статистика.

## Принятые решения
- Код чтения в пакете `reader` (как `ingest` для записи), `httpapi.Server` получает его через интерфейс `Reader`.
- Курсор это `base64url("<started_at в микросекундах>:<id>")`, запрос keyset по `(started_at, id) < (...)` на индексе `runs_started_at_id_idx`. Первая страница и страница по курсору это два sqlc-запроса: условие `cursor IS NULL OR ...` ломает использование индекса.
- Признак следующей страницы: запрашивается `limit+1` строк. Ровно полная последняя страница даёт `next_cursor = null`.
- `duration_ms` = `finished_at - started_at` в Go, только для `finished`; после resume снова `null`.
- Границы `limit`, `after_seq` и неверный `cursor` проверяет хендлер (сгенерированный код границы не проверяет) и отвечает `400`. `GET /runs/{id}/events` для неизвестного Run это `404`, а не пустой список.
- `EXPLAIN ANALYZE` на 100 тыс. запусков и 200 тыс. событий: все три запроса идут по индексам (`runs_started_at_id_idx`, `events_run_id_seq_idx`), без Seq Scan и Sort, до 0.3 мс.
- Тесты в `reader` на Postgres (testcontainers): пагинация, стабильность при новых запусках, одинаковый `started_at`, 401/400/404, плюс валидация спеки и каждого ответа через `kin-openapi`.
- Агент: `web/package.json` со скриптом `generate:api` (`openapi-typescript@7.13.0`), `make generate-ts` вызывает его. `schema.d.ts` после перегенерации не изменился.
