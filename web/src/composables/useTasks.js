import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { getStoredApiKey, setStoredApiKey } from '../utils/apiKey'

const EVENT_TYPES = [
  'task.started',
  'task.heartbeat',
  'task.succeeded',
  'task.failed',
  'task.timeout',
]

export function useTasks() {
  const tasks = reactive(new Map())
  const connected = ref(false)
  const unauthorized = ref(false)
  const apiKey = ref(getStoredApiKey())
  let eventSource = null

  function upsert(task) {
    tasks.set(task.run_id, task)
  }

  function authHeaders() {
    return apiKey.value ? { Authorization: `Bearer ${apiKey.value}` } : {}
  }

  async function loadInitial() {
    const res = await fetch('/api/v1/tasks', { headers: authHeaders() })
    if (res.status === 401) {
      unauthorized.value = true
      return
    }
    unauthorized.value = false
    if (!res.ok) return

    const body = await res.json()
    for (const task of body.tasks ?? []) {
      upsert(task)
    }
  }

  function connect() {
    eventSource?.close()

    // EventSource can't set custom headers, so an API key has to travel as a
    // query param instead — the backend accepts either.
    const url = apiKey.value
      ? `/api/v1/stream?token=${encodeURIComponent(apiKey.value)}`
      : '/api/v1/stream'

    eventSource = new EventSource(url)
    eventSource.onopen = () => {
      connected.value = true
    }
    eventSource.onerror = () => {
      connected.value = false
    }
    for (const type of EVENT_TYPES) {
      eventSource.addEventListener(type, (event) => {
        upsert(JSON.parse(event.data))
      })
    }
  }

  async function submitApiKey(key) {
    apiKey.value = key
    setStoredApiKey(key)
    await loadInitial()
    if (!unauthorized.value) {
      connect()
    }
  }

  onMounted(async () => {
    await loadInitial()
    if (!unauthorized.value) {
      connect()
    }
  })

  onUnmounted(() => {
    eventSource?.close()
  })

  const taskList = computed(() =>
    Array.from(tasks.values()).sort(
      (a, b) => new Date(b.started_at) - new Date(a.started_at),
    ),
  )

  const stats = computed(() => {
    const list = taskList.value
    const running = list.filter((t) => t.status === 'running').length
    const failed = list.filter((t) => t.status === 'failed').length
    const timeout = list.filter((t) => t.status === 'timeout').length
    const succeeded = list.filter((t) => t.status === 'success')
    const avgDurationMs = succeeded.length
      ? Math.round(
          succeeded.reduce((sum, t) => sum + (t.duration_ms ?? 0), 0) /
            succeeded.length,
        )
      : null

    return {
      total: list.length,
      running,
      failed,
      timeout,
      succeededCount: succeeded.length,
      avgDurationMs,
    }
  })

  const recentFailures = computed(() =>
    taskList.value
      .filter((t) => t.status === 'failed' || t.status === 'timeout')
      .slice(0, 10),
  )

  return {
    taskList,
    stats,
    recentFailures,
    connected,
    unauthorized,
    submitApiKey,
  }
}
