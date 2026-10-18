import { z } from 'zod'
import type { components } from './schema'

const timestamp = z.iso.datetime({ offset: true })

export const runSchema = z.object({
  id: z.uuid(),
  session_id: z.string(),
  status: z.enum(['running', 'finished']),
  cwd: z.string(),
  task_text: z.string().nullable(),
  started_at: timestamp,
  finished_at: timestamp.nullable(),
  duration_ms: z.number().int().nullable(),
})

export const runListSchema = z.object({
  items: z.array(runSchema),
  next_cursor: z.string().nullable(),
})

export const eventSchema = z.object({
  seq: z.number().int(),
  hook_event_name: z.string(),
  tool_name: z.string().nullable(),
  ts: timestamp,
  payload: z.record(z.string(), z.unknown()),
})

export const eventListSchema = z.object({
  items: z.array(eventSchema),
  next_after_seq: z.number().int().nullable(),
})

export const errorResponseSchema = z.object({
  error: z.object({ code: z.string(), message: z.string() }),
})

export type Run = z.infer<typeof runSchema>
export type RunList = z.infer<typeof runListSchema>
export type Event = z.infer<typeof eventSchema>
export type EventList = z.infer<typeof eventListSchema>

// Сверка с типами из OpenAPI: при расхождении схем tsc не соберётся.
type Same<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false
type Assert<T extends true> = T
export type SchemaChecks = [
  Assert<Same<Run, components['schemas']['Run']>>,
  Assert<Same<RunList, components['schemas']['RunList']>>,
  Assert<Same<Event, components['schemas']['Event']>>,
  Assert<Same<EventList, components['schemas']['EventList']>>,
]
