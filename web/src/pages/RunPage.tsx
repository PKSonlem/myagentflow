import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, fetchRun, fetchRunEvents } from '../api/client'
import type { Event, Run } from '../api/schemas'
import { usePaged } from '../api/usePaged'
import { ErrorBox, Status } from '../components/Status'
import { formatDuration, formatTime } from '../format'

export function RunPage() {
  const { id = '' } = useParams()
  return <RunView key={id} id={id} />
}

function RunView({ id }: { id: string }) {
  const [run, setRun] = useState<Run | null>(null)
  const [runError, setRunError] = useState<string | null>(null)

  useEffect(() => {
    const c = new AbortController()
    fetchRun(id, c.signal)
      .then(setRun)
      .catch((e: unknown) => {
        if (!c.signal.aborted) setRunError(e instanceof ApiError ? e.message : 'Неожиданная ошибка')
      })
    return () => c.abort()
  }, [id])

  const fetchPage = useCallback(
    (cursor: number | null, signal: AbortSignal) =>
      fetchRunEvents(id, { afterSeq: cursor }, signal).then((r) => ({ items: r.items, next: r.next_after_seq })),
    [id],
  )
  const events = usePaged(fetchPage)

  return (
    <>
      <p><Link to="/">← Все запуски</Link></p>
      <h1>Запуск</h1>
      {runError && <ErrorBox message={runError} />}
      {!run && !runError && <p>Загрузка…</p>}
      {run && (
        <dl>
          <dt>Статус</dt><dd><Status status={run.status} /></dd>
          <dt>cwd</dt><dd><code>{run.cwd}</code></dd>
          <dt>Задача</dt><dd>{run.task_text ?? '(нет)'}</dd>
          <dt>Начало</dt><dd>{formatTime(run.started_at)}</dd>
          <dt>Окончание</dt><dd>{run.finished_at ? formatTime(run.finished_at) : '—'}</dd>
          <dt>Длительность</dt><dd>{formatDuration(run.duration_ms)}</dd>
        </dl>
      )}

      <h2>Таймлайн</h2>
      {events.error && <ErrorBox message={events.error} />}
      <ol style={{ listStyle: 'none', padding: 0 }}>
        {events.items.map((e) => (
          <EventRow key={e.seq} event={e} />
        ))}
      </ol>
      {events.loading && <p>Загрузка…</p>}
      {!events.loading && !events.error && events.items.length === 0 && <p>Событий нет.</p>}
      {events.hasMore && !events.loading && <button onClick={events.loadMore}>Ещё</button>}
    </>
  )
}

function EventRow({ event }: { event: Event }) {
  const failed = event.hook_event_name === 'PostToolUseFailure'
  const errorText = failed && typeof event.payload.error === 'string' ? event.payload.error : null
  return (
    <li style={{ borderLeft: `4px solid ${failed ? '#b00020' : '#ccc'}`, background: failed ? '#fdecee' : undefined, padding: '4px 10px', marginBottom: 4 }}>
      <details>
        <summary>
          <small>#{event.seq} {new Date(event.ts).toLocaleTimeString('ru-RU')}</small>{' '}
          <strong>{event.hook_event_name}</strong>
          {event.tool_name && <> · {event.tool_name}</>}
          {errorText && <span style={{ color: '#b00020' }}> — {truncate(errorText, 160)}</span>}
        </summary>
        <pre style={{ overflow: 'auto', maxHeight: 400 }}>{JSON.stringify(event.payload, null, 2)}</pre>
      </details>
    </li>
  )
}

function truncate(s: string, n: number): string {
  return s.length > n ? `${s.slice(0, n)}…` : s
}
