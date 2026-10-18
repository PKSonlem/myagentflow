import { ERROR_COLOR } from './theme'

export function ErrorBox({ message }: { message: string }) {
  return (
    <p role="alert" style={{ color: ERROR_COLOR, border: `1px solid ${ERROR_COLOR}`, padding: '8px 12px' }}>
      Ошибка: {message}
    </p>
  )
}
