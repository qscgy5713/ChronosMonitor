import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'

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
  let eventSource = null

  function upsert(task) {
    tasks.set(task.run_id, task)
  }

  async function loadInitial() {
    const res = await fetch('/api/v1/tasks')
    if (!res.ok) return
    const body = await res.json()
    for (const task of body.tasks ?? []) {
      upsert(task)
    }
  }

  function connect() {
    eventSource = new EventSource('/api/v1/stream')
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

  onMounted(async () => {
    await loadInitial()
    connect()
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

  return { taskList, stats, recentFailures, connected }
}
