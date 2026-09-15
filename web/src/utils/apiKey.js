import { ref } from 'vue'

const STORAGE_KEY = 'chronos_api_key'

// localStorage can throw (private browsing, blocked site data) — never let
// that break the app, just behave as if no key were stored.
export function getStoredApiKey() {
  try {
    return localStorage.getItem(STORAGE_KEY) || ''
  } catch {
    return ''
  }
}

export function setStoredApiKey(key) {
  try {
    localStorage.setItem(STORAGE_KEY, key)
  } catch {
    // ignore
  }
}

// Shared across every composable that needs it (useTasks, useSchedules...)
// so entering a key once in the ApiKeyGate updates all of them together.
export const apiKey = ref(getStoredApiKey())

export function authHeaders() {
  return apiKey.value ? { Authorization: `Bearer ${apiKey.value}` } : {}
}
