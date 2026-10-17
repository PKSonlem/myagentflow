import { z } from 'zod'
import {
  errorResponseSchema,
  eventListSchema,
  runListSchema,
  runSchema,
  type EventList,
  type Run,
  type RunList,
} from './schemas'

export class ApiError extends Error {
  readonly status: number | null
  constructor(message: string, status: number | null = null) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

const token = import.meta.env.VITE_API_TOKEN

// Текст для пользователя; AbortError сюда не попадает, вызывающий отсекает его по signal.
export function errorMessage(e: unknown): string {
  return e instanceof ApiError ? e.message : 'Неожиданная ошибка'
}

function withQuery(path: string, params: Record<string, string | number | null | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== null && v !== undefined && v !== '') q.set(k, String(v))
  }
  return q.size > 0 ? `${path}?${q}` : path
}

async function request<S extends z.ZodType>(path: string, schema: S, signal?: AbortSignal): Promise<z.infer<S>> {
  if (!token) {
    throw new ApiError('Не задан VITE_API_TOKEN (см. web/.env.example)')
  }
  let res: Response
  try {
    res = await fetch(`/api/v1${path}`, { headers: { Authorization: `Bearer ${token}` }, signal })
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    throw new ApiError('Сервер недоступен')
  }

  let body: unknown
  try {
    body = await res.json()
  } catch {
    throw new ApiError(`Сервер вернул не JSON (HTTP ${res.status})`, res.status)
  }

  if (!res.ok) {
    const err = errorResponseSchema.safeParse(body)
    if (res.status === 404) throw new ApiError('Запуск не найден', 404)
    throw new ApiError(err.success ? err.data.error.message : `HTTP ${res.status}`, res.status)
  }

  const parsed = schema.safeParse(body)
  if (!parsed.success) {
    const issue = parsed.error.issues[0]
    const where = issue ? issue.path.join('.') || '(корень)' : '?'
    throw new ApiError(`Некорректный ответ сервера: поле ${where}: ${issue?.message ?? 'не прошло проверку'}`, res.status)
  }
  return parsed.data
}

export function fetchRuns(params: { cursor?: string | null; limit?: number } = {}, signal?: AbortSignal): Promise<RunList> {
  return request(withQuery('/runs', { cursor: params.cursor, limit: params.limit }), runListSchema, signal)
}

export function fetchRun(id: string, signal?: AbortSignal): Promise<Run> {
  return request(`/runs/${encodeURIComponent(id)}`, runSchema, signal)
}

export function fetchRunEvents(
  id: string,
  params: { afterSeq?: number | null; limit?: number } = {},
  signal?: AbortSignal,
): Promise<EventList> {
  const path = withQuery(`/runs/${encodeURIComponent(id)}/events`, { after_seq: params.afterSeq, limit: params.limit })
  return request(path, eventListSchema, signal)
}
