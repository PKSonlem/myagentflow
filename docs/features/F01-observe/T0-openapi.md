# T0: OpenAPI-контракт

**Цель:** до написания кода описать весь HTTP-контракт F01 в одной спеке, из которой генерируется серверный код и типы фронта ([ADR-0001](../../adr/0001-openapi-codegen-contract-first.md)).

**Статус:** выполнен

## Кто что пишет
- Владелец: пишет `backend/api/openapi.yaml` сам.
- Агент: в этот раз по просьбе владельца написал спеку целиком (все решения раундов 1-3 применены), владелец ревьюит и дальше правит сам. Агент задаёт вопросы по контракту, ищет пропущенные случаи (ошибки, пагинация, пустые значения, неизвестные типы событий), ревьюит схему. Спеку не пишет. Рутина: подключить линтер (`redocly lint` или `vacuum`), `oapi-codegen` и `make generate`.

## Принятые решения
- OpenAPI `3.0.3`, один файл, общие схемы в `components`.
- `operationId`: `ingestClaudeHook`, `listRuns`, `getRun`, `listRunEvents`. Схемы: `Run`, `RunList`, `Event`, `EventList`, `ClaudeHookPayload`, `ErrorResponse`, `RunStatus`.
- Формат ошибки: `{error: {code, message}}`, `code` это enum: `invalid_request`, `unauthorized`, `not_found`, `payload_too_large`, `internal_error`; один `invalid_request` на все `400`. Общие `components/responses` для `400`, `401`, `404`, `413`, `500`.
- `bearerAuth` глобально через `security`, `401` у каждого эндпоинта. `/healthz` в спеку не входит.
- Время: `format: date-time` (RFC 3339, UTC). `Event.ts` это время получения сервером, писать в `description`.
- `Run.id` это `string`, `format: uuid`. `Event` во внешнем API без `id`: идентифицируется парой (run, `seq`).
- `RunStatus`: `running | finished`.

## Что описать
1. `POST /hooks/claude` (`ingestClaudeHook`): одна терпимая схема `ClaudeHookPayload`, обязательны только `session_id` и `hook_event_name`, `additionalProperties: true`, без `oneOf`. Ответы: `200` (пустое тело; повтор события с тем же ключом идемпотентности тоже `200`), `400` (невалидный JSON или нет обязательных полей), `401`, `413` (тело больше 10 МиБ, код `payload_too_large`), `500`.
2. `GET /runs` (`listRuns`): параметры `cursor`, `limit` (по умолчанию 20, максимум 100); ответ `RunList` с `next_cursor` (непрозрачная строка, `null` на последней странице). Поля `Run`: `id`, `session_id`, `status`, `cwd`, `task_text` (nullable), `started_at`, `finished_at` (nullable), `duration_ms` (nullable, `null` пока `running`, считает сервер). Nullable-поля входят в `required`: всегда присутствуют, `null` значит «пока нет». Порядок: `started_at` и `id` по убыванию, `cursor` кодирует эту пару. Выход `limit` за пределы, битый `cursor` и не-UUID `id` это `400`. Неизвестный, но валидный `id` это `404`.
3. `GET /runs/{id}` (`getRun`): только `Run`, ответы `200`, `401`, `404`.
4. `GET /runs/{id}/events` (`listRunEvents`): параметры `after_seq`, `limit` (по умолчанию 200, максимум 1000); ответ `EventList` с `next_after_seq`. Поля `Event`: `seq`, `hook_event_name`, `tool_name` (nullable), `ts`, `payload` (свободный объект).

## Критерии приёмки
- Спека проходит линтер без ошибок.
- Для каждого эндпоинта описаны успешный ответ и все ошибки.
- `make generate` проходит без ошибок: `oapi-codegen` создаёт `internal/api/api.gen.go`.
- Из спеки генерируются TS-типы (`openapi-typescript`) без ошибок.
- Открытые вопросы по контракту записаны в конце спеки или в этом тикете, а не решены молча.

## Открытые вопросы
- `SessionStart` и http-хуки: по документации Claude Code (из сводки, не проверено) событие поддерживает только `command` и `mcp_tool`. Проверить спайком в T2. Если подтвердится, Run создаётся первым пришедшим событием (уже заложено).
- Приходит ли `cwd` во всех событиях. Сейчас он необязателен в `ClaudeHookPayload`. Решено в T1: если не пришёл, в `Run.cwd` пустая строка, она дополняется следующим событием с `cwd`. Проверить на живых хуках в T2.
- `Event.payload` отдаётся без маскирования секретов (`tool_input`, `tool_response`). Пересмотреть при выходе за localhost.
- Токен для UI лежит в `VITE_API_TOKEN` и попадает в бандл. Допустимо только на localhost; для VPS/HTTPS нужен логин. В dev используется proxy Vite на `:8980`, CORS не нужен.
- `Run.session_id` добавлен сверх первоначального списка полей (удобно сопоставлять с `claude --resume`). `source` отложен до второго источника событий.
- Лимит тела `10 МиБ` для `413` взят из T1, подтвердить на реальных payload.
- ~~Ошибка разбора параметров из `oapi-codegen` (UUID, `limit`) по умолчанию приходит не в формате `ErrorResponse`: нужен собственный error handler для строгого сервера.~~ Решено в T1: `httpapi.badRequest` для ошибок тела и параметров.

## Вне тикета
Реализация хендлеров, БД, фронт.
