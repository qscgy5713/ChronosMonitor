<script setup>
import { ref } from 'vue'

const emit = defineEmits(['submit'])

const keyInput = ref('')
const submitting = ref(false)

async function handleSubmit() {
  if (!keyInput.value.trim()) return
  submitting.value = true
  try {
    await Promise.resolve(emit('submit', keyInput.value.trim()))
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-gray-950">
    <form
      @submit.prevent="handleSubmit"
      class="w-full max-w-sm space-y-4 rounded-xl border border-gray-200 bg-white p-6 dark:border-gray-800 dark:bg-gray-900"
    >
      <div>
        <h1 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
          ChronosMonitor
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          這個伺服器已啟用 API 認證，請輸入 API key 才能查看儀表板。
        </p>
      </div>
      <input
        v-model="keyInput"
        type="password"
        placeholder="API key"
        autofocus
        class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 dark:border-gray-700 dark:bg-gray-950 dark:text-gray-100"
      />
      <button
        type="submit"
        :disabled="submitting || !keyInput.trim()"
        class="w-full rounded-lg bg-blue-600 px-3 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {{ submitting ? '驗證中…' : '確認' }}
      </button>
    </form>
  </div>
</template>
