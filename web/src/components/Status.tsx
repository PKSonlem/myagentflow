import type { Run } from '../api/schemas'

export function Status({ status }: { status: Run['status'] }) {
  return status === 'running' ? <span style={{ color: '#0a7d2c' }}>● идёт</span> : <span style={{ color: '#666' }}>завершён</span>
}

export function ErrorBox({ message }: { message: string }) {
  return (
    <p role="alert" style={{ color: '#b00020', border: '1px solid #b00020', padding: '8px 12px' }}>
      Ошибка: {message}
    </p>
  )
}
