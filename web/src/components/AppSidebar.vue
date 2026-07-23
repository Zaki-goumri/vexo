<script setup lang="ts">
import { useRouter, useRoute } from 'vue-router'
import Icon from './Icon.vue'
import { session, clearSession } from '../session'
import { logout as apiLogout } from '../api'

const router = useRouter()
const route = useRoute()

const navItems = [
  { path: '/buckets', label: 'Buckets', icon: 'bucket' as const },
  { path: '/users', label: 'Users', icon: 'user' as const },
  { path: '/keys', label: 'Access Keys', icon: 'key' as const },
  { path: '/policies', label: 'Policies', icon: 'shield' as const },
]

function isActive(path: string) {
  return route.path === path || route.path.startsWith(path + '/')
}

async function logout() {
  await apiLogout()
  clearSession()
  router.push('/login')
}
</script>

<template>
  <aside style="width: 232px; background: var(--sidebar-bg); display: flex; flex-direction: column; flex-shrink: 0;">
    <div style="display: flex; align-items: center; gap: 10px; padding: 20px 20px 16px;">
      <div style="width: 28px; height: 28px; border-radius: 6px; background: var(--accent); display: flex; align-items: center; justify-content: center; color: #fff; font-weight: 700; font-size: 14px;">V</div>
      <div style="font-size: 17px; font-weight: 700; color: #fff; letter-spacing: 0.3px;">vexo</div>
    </div>
    <nav style="flex: 1; padding: 8px 0;">
      <a v-for="item in navItems" :key="item.path"
        @click="router.push(item.path)"
        :style="{
          display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 20px', cursor: 'pointer',
          color: isActive(item.path) ? 'var(--sidebar-active)' : 'var(--sidebar-text)',
          background: isActive(item.path) ? 'var(--sidebar-surface)' : 'transparent',
          borderLeft: isActive(item.path) ? '3px solid var(--accent)' : '3px solid transparent',
          fontSize: '13px', fontWeight: 500,
        }">
        <Icon :name="item.icon" :size="16" />
        {{ item.label }}
      </a>
    </nav>
    <div style="padding: 14px 20px; border-top: 1px solid var(--sidebar-border);">
      <div v-if="session.username" style="color: var(--sidebar-text-dim); font-size: 12px; margin-bottom: 10px;">
        Signed in as <strong style="color: var(--sidebar-text);">{{ session.username }}</strong>
      </div>
      <a @click="logout" style="display: flex; align-items: center; gap: 10px; cursor: pointer; color: var(--sidebar-text); font-size: 13px;">
        <Icon name="logout" :size="15" />
        Logout
      </a>
    </div>
  </aside>
</template>
