import type { Paged } from '../api/usePaged'
import { ErrorBox } from './ErrorBox'

// Ошибка, загрузка, пустой список и кнопка «Ещё» для любого постраничного списка.
export function PagedStatus<T>({ paged, empty }: { paged: Paged<T>; empty: string }) {
  const { items, loading, error, hasMore, loadMore } = paged
  return (
    <>
      {error && <ErrorBox message={error} />}
      {loading && <p>Загрузка…</p>}
      {!loading && !error && items.length === 0 && <p>{empty}</p>}
      {hasMore && !loading && <button onClick={loadMore}>Ещё</button>}
    </>
  )
}
