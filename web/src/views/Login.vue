<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login } from '../api'
import logo from '../assets/logo-mark.png'
import Icon from '../components/Icon.vue'

const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

const FEATURES = [
  { icon: 'bucket' as const, text: 'S3-compatible HTTP API with SigV4 auth' },
  { icon: 'shield' as const, text: 'AWS IAM-style policies, per user or group' },
  { icon: 'key' as const, text: 'Access keys and full user management' },
  { icon: 'upload' as const, text: 'Automatic hot / warm / cold tiering' },
]

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await login(username.value, password.value)
    router.push('/buckets')
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <aside class="login-brand">
      <div class="login-brand-inner">
        <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 28px;">
          <img :src="logo" alt="vexo" style="width: 40px; height: 40px; object-fit: contain;" />
          <span style="font-size: 22px; font-weight: 700; color: #fff;">vexo</span>
        </div>
        <h1 style="font-size: 28px; font-weight: 700; color: #fff; line-height: 1.3; margin-bottom: 16px;">
          Self-hosted object storage,<br />built on a single Go binary.
        </h1>
        <p style="color: var(--sidebar-text-dim); font-size: 14px; margin-bottom: 36px;">
          One embedded metadata store. No external database, no cluster to run.
        </p>
        <ul class="login-features">
          <li v-for="f in FEATURES" :key="f.text">
            <span class="login-feature-icon"><Icon :name="f.icon" :size="15" /></span>
            {{ f.text }}
          </li>
        </ul>
      </div>
    </aside>

    <main class="login-form-panel">
      <div class="card" style="width: 380px;">
        <div class="login-form-logo">
          <img :src="logo" alt="vexo" style="width: 40px; height: 40px; object-fit: contain; margin-bottom: 12px;" />
          <h1 style="font-size: 20px; font-weight: 700;">vexo</h1>
        </div>
        <h2 style="font-size: 17px; font-weight: 700; margin-bottom: 4px;">Sign in</h2>
        <p style="color: var(--text2); font-size: 13px; margin-bottom: 24px;">Use your console credentials to continue.</p>
        <form @submit.prevent="submit">
          <div style="margin-bottom: 16px;">
            <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Username</label>
            <input v-model="username" type="text" placeholder="Enter username" required autofocus />
          </div>
          <div style="margin-bottom: 20px;">
            <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Password</label>
            <input v-model="password" type="password" placeholder="Enter password" required />
          </div>
          <div v-if="error" style="color: var(--danger); margin-bottom: 12px; font-size: 13px;">{{ error }}</div>
          <button type="submit" class="primary" style="width: 100%;" :disabled="loading">
            {{ loading ? 'Signing in...' : 'Sign In' }}
          </button>
        </form>
      </div>
    </main>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  height: 100vh;
}

.login-brand {
  flex: 1;
  background: var(--sidebar-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 60px;
}

.login-brand-inner {
  max-width: 440px;
}

.login-features {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.login-features li {
  display: flex;
  align-items: center;
  gap: 14px;
  color: var(--sidebar-text);
  font-size: 14px;
}

.login-feature-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 8px;
  background: var(--sidebar-surface);
  color: var(--accent);
}

.login-form-panel {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg);
}

.login-form-logo {
  display: none;
  flex-direction: column;
  align-items: center;
  margin-bottom: 24px;
}

@media (max-width: 860px) {
  .login-brand {
    display: none;
  }
  .login-form-logo {
    display: flex;
  }
}
</style>
