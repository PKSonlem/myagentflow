import type { Run } from '../api/schemas'

export function Status({ status }: { status: Run['status'] }) {
  return status === 'running' ? <span style={{ color: '#0a7d2c' }}>● идёт</span> : <span style={{ color: '#666' }}>завершён</span>
}
