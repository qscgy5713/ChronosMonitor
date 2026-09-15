<script setup>
defineProps({
  status: { type: String, required: true },
})

const styles = {
  running: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-blue-500/20',
  success:
    'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-emerald-500/20',
  failed: 'bg-red-500/10 text-red-600 dark:text-red-400 ring-red-500/20',
  timeout:
    'bg-amber-500/10 text-amber-600 dark:text-amber-400 ring-amber-500/20',
  ok: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-emerald-500/20',
  missed: 'bg-red-500/10 text-red-600 dark:text-red-400 ring-red-500/20',
}

const labels = {
  running: '執行中',
  success: '成功',
  failed: '失敗',
  timeout: '逾時',
  ok: '正常',
  missed: '錯過',
}
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset"
    :class="styles[status] ?? 'bg-gray-500/10 text-gray-600 ring-gray-500/20'"
  >
    <span
      v-if="status === 'running'"
      class="h-1.5 w-1.5 animate-pulse rounded-full bg-blue-500"
    />
    {{ labels[status] ?? status }}
  </span>
</template>
