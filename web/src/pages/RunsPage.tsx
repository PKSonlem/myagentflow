import { useCallback } from 'react'
import { Link } from 'react-router-dom'
import { fetchRuns } from '../api/client'
import { usePaged } from '../api/usePaged'
import { ErrorBox, Status } from '../components/Status'
import { formatDuration, formatTime } from '../format'

export function RunsPage() {
  const fetchPage = useCallback(
    (cursor: string | null, signal: AbortSignal) =>
      fetchRuns({ cursor }, signal).then((r) => ({ items: r.items, next: r.next_cursor })),
    [],
  )
  const { items, loading, error, hasMore, loadMore } = usePaged(fetchPage)

  return (
    <>
      <h1>Запуски</h1>
      {error && <ErrorBox message={error} />}
      {items.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Статус</th>
              <th>cwd</th>
              <th>Задача</th>
              <th>Начало</th>
              <th>Длительность</th>
            </tr>
          </thead>
          <tbody>
            {items.map((r) => (
              <tr key={r.id}>
                <td><Status status={r.status} /></td>
                <td><code>{r.cwd}</code></td>
                <td><Link to={`/runs/${r.id}`}>{r.task_text ? truncate(r.task_text, 120) : '(без текста)'}</Link></td>
                <td>{formatTime(r.started_at)}</td>
                <td>{formatDuration(r.duration_ms)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {loading && <p>Загрузка…</p>}
      {!loading && !error && items.length === 0 && <p>Запусков пока нет.</p>}
      {hasMore && !loading && <button onClick={loadMore}>Ещё</button>}
    </>
  )
}

function truncate(s: string, n: number): string {
  return s.length > n ? `${s.slice(0, n)}…` : s
}
