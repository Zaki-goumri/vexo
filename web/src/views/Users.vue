<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api'

interface User {
  Username: string
  Status: string
  Groups: string[]
  Policies: string[]
  CreatedAt: string
}
const users = ref<User[]>([])
const error = ref('')
const showCreate = ref(false)
const newName = ref('')

async function load() {
  try {
    users.value = await api.get<User[]>('/users')
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

async function create() {
  error.value = ''
  try {
    await api.post('/users', { username: newName.value })
    showCreate.value = false
    newName.value = ''
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}

async function remove(name: string) {
  if (!confirm(`Delete user "${name}"? This also deletes their access keys.`)) return
  try {
    await api.del(`/users/${name}`)
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
      <h2 style="font-size: 20px;">Users</h2>
      <button class="primary" @click="showCreate = true">Create User</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div class="card">
      <table v-if="users.length > 0">
        <thead>
          <tr><th>Username</th><th>Status</th><th>Policies</th><th>Groups</th><th>Actions</th></tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.Username">
            <td>{{ u.Username }}</td>
            <td>
              <span :style="{ padding: '2px 8px', borderRadius: '4px', fontSize: '11px', background: u.Status === 'active' ? 'rgba(34,197,94,0.2)' : 'rgba(239,68,68,0.2)', color: u.Status === 'active' ? 'var(--success)' : 'var(--danger)' }">{{ u.Status }}</span>
            </td>
            <td style="color: var(--text2);">{{ u.Policies?.join(', ') || '-' }}</td>
            <td style="color: var(--text2);">{{ u.Groups?.join(', ') || '-' }}</td>
            <td><button class="danger" @click="remove(u.Username)">Delete</button></td>
          </tr>
        </tbody>
      </table>
      <div v-else style="color: var(--text2); text-align: center; padding: 24px;">No users yet</div>
    </div>

    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="card" style="width: 400px;">
        <h3 style="margin-bottom: 16px;">Create User</h3>
        <input v-model="newName" placeholder="username" @keyup.enter="create" style="margin-bottom: 16px;" />
        <div style="display: flex; gap: 8px;">
          <button class="secondary" @click="showCreate = false" style="flex: 1;">Cancel</button>
          <button class="primary" @click="create" style="flex: 1;">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>