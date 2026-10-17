export function formatTime(iso: string): string {
  return new Date(iso).toLocaleString('ru-RU')
}

export function formatDuration(ms: number | null): string {
  if (ms === null) return '—'
  if (ms < 1000) return `${ms} мс`
  const s = Math.round(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h} ч ${m} мин`
  if (m > 0) return `${m} мин ${sec} с`
  return `${sec} с`
}
