export function formatDuration(ms) {
  if (ms == null) return '—'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

export function formatTime(iso) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('zh-TW', { hour12: false })
}

export function shortId(id) {
  return id.length > 8 ? `${id.slice(0, 8)}…` : id
}
