# MyAgentFlow

Панель наблюдения за запусками Claude Code: принимает события хуков и показывает, что делал агент.

## Language

**Run**:
Один запуск агента, соответствующий одной сессии Claude Code (`session_id`). Появляется с первым событием сессии и находится в `running`, пока не придёт `SessionEnd`, после которого становится `finished`; возобновлённая сессия снова открывает тот же Run первым же событием.
_Avoid_: Session, job, task

**Event**:
Одна запись хронологии Run, соответствующая одному вызову хука Claude Code; упорядочена внутри Run номером `seq`, который назначает сервер.
_Avoid_: Step, log entry, message

**Hook payload**:
Нативный JSON, который Claude Code присылает хуком; его формат задаёт Claude Code, а не мы.
_Avoid_: Webhook, request body
