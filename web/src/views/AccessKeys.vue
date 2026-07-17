<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api'

interface User {
  Username: string
  Status: string
}
interface KeyInfo {
  accessKey: string
  username: string
  status: string
  createdAt: string
}
interface CreatedKey {
  accessKey: string
  secret: string
}

const users = ref<User[]>([])
const allKeys = ref<KeyInfo[]>([])
const error = ref('')
const showCreate = ref(false)
const selectedUser = ref('')
const createdKey = ref<CreatedKey | null>(null)

async function load() {
  try {
    users.value = await api.get<User[]>('/users')
    allKeys.value = []
    for (const u of users.value) {
      const keys = await api.get<KeyInfo[]>(`/users/${u.Username}/keys`)
      allKeys.value.push(...keys)
    }
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

async function createKey() {
  error.value = ''
  try {
    createdKey.value = await api.post<CreatedKey>(`/users/${selectedUser.value}/keys`)
    showCreate.value = false
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}

async function removeKey(id: string) {
  if (!confirm(`Delete access key "${id}"?`)) return
  try {
    await api.del(`/keys/${id}`)
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
      <h2 style="font-size: 20px;">Access Keys</h2>
      <button class="primary" @click="showCreate = true">Create Access Key</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div class="card">
      <table v-if="allKeys.length > 0">
        <thead>
          <tr><th>Access Key</th><th>User</th><th>Status</th><th>Created</th><th>Actions</th></tr>
        </thead>
        <tbody>
          <tr v-for="k in allKeys" :key="k.accessKey">
            <td style="font-family: monospace; font-size: 12px;">{{ k.accessKey }}</td>
            <td>{{ k.username }}</td>
            <td>
              <span :style="{ padding: '2px 8px', borderRadius: '4px', fontSize: '11px', background: k.status === 'active' ? 'rgba(34,197,94,0.2)' : 'rgba(239,68,68,0.2)', color: k.status === 'active' ? 'var(--success)' : 'var(--danger)' }">{{ k.status }}</span>
            </td>
            <td style="color: var(--text2);">{{ k.createdAt }}</td>
            <td><button class="danger" @click="removeKey(k.accessKey)">Delete</button></td>
          </tr>
        </tbody>
      </table>
      <div v-else style="color: var(--text2); text-align: center; padding: 24px;">No access keys</div>
    </div>

    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="card" style="width: 400px;">
        <h3 style="margin-bottom: 16px;">Create Access Key</h3>
        <div style="margin-bottom: 16px;">
          <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">User</label>
          <select v-model="selectedUser" style="width: 100%; padding: 8px; background: var(--surface2); color: var(--text); border: 1px solid var(--border); border-radius: var(--radius);">
            <option v-for="u in users" :key="u.Username" :value="u.Username">{{ u.Username }}</option>
          </select>
        </div>
        <div style="display: flex; gap: 8px;">
          <button class="secondary" @click="showCreate = false" style="flex: 1;">Cancel</button>
          <button class="primary" @click="createKey" style="flex: 1;" :disabled="!selectedUser">Create</button>
        </div>
      </div>
    </div>

    <div v-if="createdKey" class="modal-overlay" @click.self="createdKey = null">
      <div class="card" style="width: 460px;">
        <h3 style="margin-bottom: 8px;">Access Key Created</h3>
        <p style="color: var(--danger); font-size: 12px; margin-bottom: 16px;">⚠ Save these credentials now — the secret will not be shown again.</p>
        <div style="background: var(--surface2); padding: 12px; border-radius: var(--radius); margin-bottom: 16px;">
          <div style="margin-bottom: 8px;">
            <span style="color: var(--text2); font-size: 12px;">Access Key:</span>
            <code style="display: block; margin-top: 4px; font-family: monospace; word-break: break-all;">{{ createdKey.accessKey }}</code>
          </div>
          <div>
            <span style="color: var(--text2); font-size: 12px;">Secret:</span>
            <code style="display: block; margin-top: 4px; font-family: monospace; word-break: break-all; color: var(--accent);">{{ createdKey.secret }}</code>
          </div>
        </div>
        <button class="primary" style="width: 100%;" @click="createdKey = null">I've saved them</button>
      </div>
    </div>
  </div>
</template>