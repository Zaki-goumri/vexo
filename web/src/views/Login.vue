<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login } from '../api'

const router = useRouter()
const accessKey = ref('')
const secret = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await login(accessKey.value, secret.value)
    router.push('/buckets')
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div style="display: flex; align-items: center; justify-content: center; height: 100vh; background: var(--sidebar-bg);">
    <div class="card" style="width: 380px;">
      <div style="display: flex; flex-direction: column; align-items: center; margin-bottom: 24px;">
        <div style="width: 44px; height: 44px; border-radius: 8px; background: var(--accent); display: flex; align-items: center; justify-content: center; color: #fff; font-weight: 700; font-size: 20px; margin-bottom: 12px;">V</div>
        <h1 style="font-size: 20px; font-weight: 700;">vexo</h1>
      </div>
      <form @submit.prevent="submit">
        <div style="margin-bottom: 16px;">
          <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Access Key</label>
          <input v-model="accessKey" type="text" placeholder="Enter access key" required />
        </div>
        <div style="margin-bottom: 20px;">
          <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Secret Key</label>
          <input v-model="secret" type="password" placeholder="Enter secret key" required />
        </div>
        <div v-if="error" style="color: var(--danger); margin-bottom: 12px; font-size: 13px;">{{ error }}</div>
        <button type="submit" class="primary" style="width: 100%;" :disabled="loading">
          {{ loading ? 'Signing in...' : 'Sign In' }}
        </button>
      </form>
    </div>
  </div>
</template>
