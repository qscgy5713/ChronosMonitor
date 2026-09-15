import { computed, onMounted, onUnmounted, reactive } from 'vue'
import { authHeaders } from '../utils/apiKey'

const POLL_INTERVAL_MS = 15000

// Schedule status changes aren't as time-critical as live task events, and
// a "back to ok" transition (from a fresh start report) has no dedicated
// broadcast event today — so this polls instead of relying purely on SSE.
export function useSchedules() {
  const schedules = reactive(new Map())
  let pollTimer = null

  function upsert(schedule) {
    schedules.set(schedule.task_name, schedule)
  }

  async function refresh() {
    const res = await fetch('/api/v1/schedules', { headers: authHeaders() })
    if (!res.ok) return

    const body = await res.json()
    const seen = new Set()
    for (const schedule of body.schedules ?? []) {
      upsert(schedule)
      seen.add(schedule.task_name)
    }
    for (const taskName of schedules.keys()) {
      if (!seen.has(taskName)) schedules.delete(taskName)
    }
  }

  async function remove(taskName) {
    const res = await fetch(`/api/v1/schedules/${encodeURIComponent(taskName)}`, {
      method: 'DELETE',
      headers: authHeaders(),
    })
    if (res.ok) {
      schedules.delete(taskName)
    }
    return res.ok
  }

  onMounted(() => {
    refresh()
    pollTimer = setInterval(refresh, POLL_INTERVAL_MS)
  })

  onUnmounted(() => {
    clearInterval(pollTimer)
  })

  const scheduleList = computed(() =>
    Array.from(schedules.values()).sort((a, b) =>
      a.task_name.localeCompare(b.task_name),
    ),
  )

  const missedCount = computed(
    () => scheduleList.value.filter((s) => s.status === 'missed').length,
  )

  return { scheduleList, missedCount, refresh, remove }
}
