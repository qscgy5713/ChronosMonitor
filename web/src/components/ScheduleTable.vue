<script setup>
import { ref } from 'vue'
import StatusBadge from './StatusBadge.vue'
import { formatCadence, formatTime } from '../utils/format'

const props = defineProps({
  schedules: { type: Array, required: true },
})

const emit = defineEmits(['delete'])

const deletingTaskName = ref(null)

async function handleDelete(taskName) {
  deletingTaskName.value = taskName
  try {
    await Promise.resolve(emit('delete', taskName))
  } finally {
    deletingTaskName.value = null
  }
}
</script>

<template>
  <div
    class="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900"
  >
    <table class="w-full min-w-[640px] text-left text-sm">
      <thead>
        <tr
          class="border-b border-gray-200 text-gray-500 dark:border-gray-800 dark:text-gray-400"
        >
          <th class="px-4 py-3 font-medium">任務名稱</th>
          <th class="px-4 py-3 font-medium">預期排程</th>
          <th class="px-4 py-3 font-medium">緩衝</th>
          <th class="px-4 py-3 font-medium">狀態</th>
          <th class="px-4 py-3 font-medium">最後回報</th>
          <th class="px-4 py-3 font-medium"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="props.schedules.length === 0">
          <td colspan="6" class="px-4 py-8 text-center text-gray-400">
            尚未註冊任何 missed-run 排程
          </td>
        </tr>
        <tr
          v-for="schedule in props.schedules"
          :key="schedule.task_name"
          class="border-b border-gray-100 last:border-0 dark:border-gray-800/60"
        >
          <td class="px-4 py-3 font-medium">{{ schedule.task_name }}</td>
          <td class="px-4 py-3 text-gray-500 dark:text-gray-400">
            {{ formatCadence(schedule) }}
          </td>
          <td class="px-4 py-3 text-gray-500 dark:text-gray-400">
            {{ schedule.grace_period_seconds }}s
          </td>
          <td class="px-4 py-3"><StatusBadge :status="schedule.status" /></td>
          <td class="px-4 py-3 text-gray-500 dark:text-gray-400">
            {{ formatTime(schedule.last_seen_at) }}
          </td>
          <td class="px-4 py-3 text-right">
            <button
              type="button"
              :disabled="deletingTaskName === schedule.task_name"
              @click="handleDelete(schedule.task_name)"
              class="text-xs text-gray-400 hover:text-red-600 disabled:opacity-50 dark:hover:text-red-400"
            >
              取消追蹤
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
