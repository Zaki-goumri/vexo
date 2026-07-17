<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '../api'

const router = useRouter()
const route = useRoute()
const username = ref('')

onMounted(async () => {
  try {
    const data = await api.get<{authenticated: boolean; username?: string}>('/session')
    username.value = data.username || ''
  } catch {}
})

async function logout() {
  await api.post('/logout')
  router.push('/login')
}

const navItems = [
  { path: '/buckets', label: 'Buckets' },
  { path: '/users', label: 'Users' },
  { path: '/keys', label: 'Access Keys' },
  { path: '/policies', label: 'Policies' },
]
</script>

<template>
  <div style="display: flex; height: 100vh;">
    <aside style="width: 220px; background: var(--surface); border-right: 1px solid var(--border); padding: 16px 0;">
      <div style="font-size: 18px; font-weight: 700; padding: 8px 20px; margin-bottom: 20px;">Vexo</div>
      <nav>
        <a v-for="item in navItems" :key="item.path"
          @click="router.push(item.path)"
          :style="{
            display: 'block', padding: '10px 20px', cursor: 'pointer',
            color: route.path === item.path || route.path.startsWith(item.path + '/') ? 'var(--accent)' : 'var(--text2)',
            background: route.path === item.path || route.path.startsWith(item.path + '/') ? 'var(--surface2)' : 'transparent',
            borderLeft: route.path === item.path || route.path.startsWith(item.path + '/') ? '3px solid var(--accent)' : '3px solid transparent',
          }">
          {{ item.label }}
        </a>
      </nav>
      <div v-if="username" style="position: absolute; bottom: 16px; left: 16px; right: 16px;">
        <div style="color: var(--text2); font-size: 12px; margin-bottom: 8px;">Signed in as {{ username }}</div>
        <button class="secondary" style="width: 100%;" @click="logout">Logout</button>
      </div>
    </aside>
    <main style="flex: 1; padding: 32px; overflow-y: auto;">
      <router-view />
    </main>
  </div>
</template>