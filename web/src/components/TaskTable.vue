<script setup>
import StatusBadge from './StatusBadge.vue'
import { formatDuration, formatTime, shortId } from '../utils/format'

defineProps({
  tasks: { type: Array, required: true },
})
</script>

<template>
  <div
    class="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900"
  >
    <table class="w-full min-w-[720px] text-left text-sm">
      <thead>
        <tr
          class="border-b border-gray-200 text-gray-500 dark:border-gray-800 dark:text-gray-400"
        >
          <th class="px-4 py-3 font-medium">任務名稱</th>
          <th class="px-4 py-3 font-medium">Run ID</th>
          <th class="px-4 py-3 font-medium">狀態</th>
          <th class="px-4 py-3 font-medium">開始時間</th>
          <th class="px-4 py-3 font-medium">耗時</th>
          <th class="px-4 py-3 font-medium">來源</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="tasks.length === 0">
          <td colspan="6" class="px-4 py-8 text-center text-gray-400">
            尚無任務回報
          </td>
        </tr>
        <tr
          v-for="task in tasks"
          :key="task.run_id"
          class="border-b border-gray-100 last:border-0 dark:border-gray-800/60"
        >
          <td class="px-4 py-3 font-medium">{{ task.task_name }}</td>
          <td class="px-4 py-3 font-mono text-xs text-gray-500 dark:text-gray-400">
            {{ shortId(task.run_id) }}
          </td>
          <td class="px-4 py-3"><StatusBadge :status="task.status" /></td>
          <td class="px-4 py-3 text-gray-500 dark:text-gray-400">
            {{ formatTime(task.started_at) }}
          </td>
          <td class="px-4 py-3 text-gray-500 dark:text-gray-400">
            {{ formatDuration(task.duration_ms) }}
          </td>
          <td class="px-4 py-3 text-gray-500 dark:text-gray-400">
            {{ task.source || '—' }}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
