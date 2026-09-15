<script setup>
import { useTasks } from './composables/useTasks'
import { useSchedules } from './composables/useSchedules'
import StatCard from './components/StatCard.vue'
import TaskTable from './components/TaskTable.vue'
import FailurePanel from './components/FailurePanel.vue'
import ScheduleTable from './components/ScheduleTable.vue'
import ApiKeyGate from './components/ApiKeyGate.vue'
import { formatDuration } from './utils/format'

const {
  taskList,
  stats,
  recentFailures,
  connected,
  unauthorized,
  submitApiKey,
} = useTasks()

const { scheduleList, missedCount, remove: removeSchedule } = useSchedules()
</script>

<template>
  <ApiKeyGate v-if="unauthorized" @submit="submitApiKey" />

  <div
    v-else
    class="min-h-screen bg-gray-50 text-gray-900 dark:bg-gray-950 dark:text-gray-100"
  >
    <header
      class="border-b border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900"
    >
      <div class="mx-auto flex max-w-6xl items-center justify-between px-4 py-4">
        <h1 class="text-lg font-semibold">ChronosMonitor</h1>
        <div class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
          <span
            class="h-2 w-2 rounded-full"
            :class="connected ? 'bg-emerald-500' : 'bg-gray-400'"
          />
          {{ connected ? '即時連線中' : '連線中斷' }}
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-6xl space-y-6 px-4 py-6">
      <section class="grid grid-cols-2 gap-4 sm:grid-cols-5">
        <StatCard label="執行中" :value="stats.running" tone="blue" />
        <StatCard label="成功" :value="stats.succeededCount" tone="emerald" />
        <StatCard label="失敗" :value="stats.failed" tone="red" />
        <StatCard label="逾時" :value="stats.timeout" tone="amber" />
        <StatCard
          label="平均耗時（成功）"
          :value="formatDuration(stats.avgDurationMs)"
        />
      </section>

      <FailurePanel :tasks="recentFailures" />

      <section class="space-y-3">
        <div class="flex items-center gap-2">
          <h2 class="text-sm font-semibold text-gray-700 dark:text-gray-300">
            排程監控（Missed Run）
          </h2>
          <span
            v-if="missedCount > 0"
            class="rounded-full bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-400"
          >
            {{ missedCount }} 個錯過
          </span>
        </div>
        <ScheduleTable :schedules="scheduleList" @delete="removeSchedule" />
      </section>

      <TaskTable :tasks="taskList" />
    </main>
  </div>
</template>
