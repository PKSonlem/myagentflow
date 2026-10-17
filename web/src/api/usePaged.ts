import { useCallback, useEffect, useRef, useState } from 'react'
import { errorMessage } from './client'

interface Page<T, C> {
  items: T[]
  next: C | null
}

export interface Paged<T> {
  items: T[]
  loading: boolean
  error: string | null
  hasMore: boolean
  loadMore: () => void
}

// Загружает первую страницу при монтировании и дальше по кнопке «ещё».
// fetchPage должна быть стабильной (useCallback); при смене источника компонент перемонтируют через key.
export function usePaged<T, C>(fetchPage: (cursor: C | null, signal: AbortSignal) => Promise<Page<T, C>>): Paged<T> {
  const [items, setItems] = useState<T[]>([])
  const [next, setNext] = useState<C | null>(null)
  const [loaded, setLoaded] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const ctrl = useRef<AbortController | null>(null)

  // Состояние начальное (loading = true), поэтому первая загрузка его не сбрасывает.
  const fetchInto = useCallback(
    (cursor: C | null, append: boolean) => {
      ctrl.current?.abort()
      const c = new AbortController()
      ctrl.current = c
      fetchPage(cursor, c.signal)
        .then((p) => {
          setItems((prev) => (append ? [...prev, ...p.items] : p.items))
          setNext(p.next)
          setLoaded(true)
        })
        .catch((e: unknown) => {
          if (c.signal.aborted) return
          setError(errorMessage(e))
        })
        .finally(() => {
          if (!c.signal.aborted) setLoading(false)
        })
    },
    [fetchPage],
  )

  useEffect(() => {
    fetchInto(null, false)
    return () => ctrl.current?.abort()
  }, [fetchInto])

  const loadMore = useCallback(() => {
    if (next === null) return
    setLoading(true)
    setError(null)
    fetchInto(next, true)
  }, [fetchInto, next])

  return { items, loading, error, hasMore: loaded && next !== null, loadMore }
}
