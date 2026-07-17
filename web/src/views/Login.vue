<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const accessKey = ref('')
const secret = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await api.post('/login', { accessKey: accessKey.value, secret: secret.value })
    router.push('/buckets')
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div style="display: flex; align-items: center; justify-content: center; height: 100vh;">
    <div class="card" style="width: 380px;">
      <h1 style="font-size: 24px; margin-bottom: 24px; text-align: center;">Vexo Console</h1>
      <form @submit.prevent="submit">
        <div style="margin-bottom: 16px;">
          <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Access Key</label>
          <input v-model="accessKey" type="text" placeholder="Enter access key" required />
        </div>
        <div style="margin-bottom: 20px;">
          <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Secret</label>
          <input v-model="secret" type="password" placeholder="Enter secret" required />
        </div>
        <div v-if="error" style="color: var(--danger); margin-bottom: 12px; font-size: 13px;">{{ error }}</div>
        <button type="submit" class="primary" style="width: 100%;" :disabled="loading">
          {{ loading ? 'Signing in...' : 'Sign In' }}
        </button>
      </form>
    </div>
  </div>
</template>