<script setup>
const props = defineProps({
  label: { type: String, required: true },
  value: { type: [String, Number], required: true },
  tone: { type: String, default: 'default' },
})

const toneClasses = {
  default: 'text-gray-900 dark:text-gray-100',
  blue: 'text-blue-600 dark:text-blue-400',
  emerald: 'text-emerald-600 dark:text-emerald-400',
  red: 'text-red-600 dark:text-red-400',
  amber: 'text-amber-600 dark:text-amber-400',
}
</script>

<template>
  <div
    class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
  >
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ label }}</p>
    <p class="mt-1 text-2xl font-semibold" :class="toneClasses[props.tone]">
      {{ value }}
    </p>
  </div>
</template>
