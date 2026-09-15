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

export function formatIntervalSeconds(seconds) {
  if (seconds % 86400 === 0) return `每 ${seconds / 86400} 天`
  if (seconds % 3600 === 0) return `每 ${seconds / 3600} 小時`
  if (seconds % 60 === 0) return `每 ${seconds / 60} 分鐘`
  return `每 ${seconds} 秒`
}

export function formatCadence(schedule) {
  if (schedule.cron_expression) return `cron: ${schedule.cron_expression}`
  if (schedule.expected_interval_seconds != null) {
    return formatIntervalSeconds(schedule.expected_interval_seconds)
  }
  return '—'
}
