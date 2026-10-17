import { useCallback } from 'react'
import { Link } from 'react-router-dom'
import { fetchRuns } from '../api/client'
import { usePaged } from '../api/usePaged'
import { PagedStatus } from '../components/PagedStatus'
import { Status } from '../components/Status'
import { formatDuration, formatTime, truncate } from '../format'

export function RunsPage() {
  const fetchPage = useCallback(
    (cursor: string | null, signal: AbortSignal) =>
      fetchRuns({ cursor }, signal).then((r) => ({ items: r.items, next: r.next_cursor })),
    [],
  )
  const runs = usePaged(fetchPage)

  return (
    <>
      <h1>Запуски</h1>
      {runs.items.length > 0 && (
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
            {runs.items.map((r) => (
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
      <PagedStatus paged={runs} empty="Запусков пока нет." />
    </>
  )
}
