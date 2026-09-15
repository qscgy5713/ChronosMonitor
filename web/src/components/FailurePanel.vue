<script setup>
import StatusBadge from './StatusBadge.vue'
import { formatTime } from '../utils/format'

defineProps({
  tasks: { type: Array, required: true },
})
</script>

<template>
  <div
    class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
  >
    <h2 class="mb-3 text-sm font-semibold text-gray-700 dark:text-gray-300">
      近期失敗 / 逾時任務
    </h2>
    <p v-if="tasks.length === 0" class="text-sm text-gray-400">
      目前沒有失敗或逾時的任務 🎉
    </p>
    <ul v-else class="space-y-3">
      <li
        v-for="task in tasks"
        :key="task.run_id"
        class="rounded-lg border border-gray-100 p-3 dark:border-gray-800"
      >
        <div class="flex items-center justify-between gap-2">
          <span class="font-medium">{{ task.task_name }}</span>
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-400">{{
              formatTime(task.finished_at)
            }}</span>
            <StatusBadge :status="task.status" />
          </div>
        </div>
        <pre
          v-if="task.error_message"
          class="mt-2 overflow-x-auto rounded-md bg-gray-50 p-2 text-xs text-red-600 dark:bg-gray-950 dark:text-red-400"
        ><code>{{ task.error_message }}</code></pre>
      </li>
    </ul>
  </div>
</template>
