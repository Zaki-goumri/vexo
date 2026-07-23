<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listUsers, listKeys, createKey, deleteKey, type User, type KeyInfo, type CreatedKey } from '../api'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import Badge from '../components/Badge.vue'
import EmptyState from '../components/EmptyState.vue'

const users = ref<User[]>([])
const allKeys = ref<KeyInfo[]>([])
const error = ref('')
const showCreate = ref(false)
const selectedUser = ref('')
const createdKey = ref<CreatedKey | null>(null)

async function load() {
  try {
    users.value = await listUsers()
    allKeys.value = []
    for (const u of users.value) {
      const keys = await listKeys(u.username)
      allKeys.value.push(...keys)
    }
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

async function create() {
  error.value = ''
  try {
    createdKey.value = await createKey(selectedUser.value)
    showCreate.value = false
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}

async function removeKey(id: string) {
  if (!confirm(`Delete access key "${id}"?`)) return
  try {
    await deleteKey(id)
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
      <button class="primary" @click="showCreate = true">Create Access Key +</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div class="card" style="padding: 0;">
      <table v-if="allKeys.length > 0">
        <thead>
          <tr><th>Access Key</th><th>User</th><th>Status</th><th>Created</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="k in allKeys" :key="k.accessKey">
            <td style="display: flex; align-items: center; gap: 10px; font-family: monospace; font-size: 12px;"><Icon name="key" :size="14" /> {{ k.accessKey }}</td>
            <td>{{ k.username }}</td>
            <td><Badge :tone="k.status === 'active' ? 'success' : 'danger'">{{ k.status }}</Badge></td>
            <td style="color: var(--text2);">{{ k.createdAt }}</td>
            <td style="text-align: right;"><button class="danger" @click="removeKey(k.accessKey)"><Icon name="trash" :size="14" /></button></td>
          </tr>
        </tbody>
      </table>
      <EmptyState v-else text="No access keys" />
    </div>

    <Modal v-if="showCreate" title="Create Access Key" @close="showCreate = false">
      <div style="margin-bottom: 16px;">
        <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">User</label>
        <select v-model="selectedUser">
          <option v-for="u in users" :key="u.username" :value="u.username">{{ u.username }}</option>
        </select>
      </div>
      <template #footer>
        <button class="secondary" @click="showCreate = false" style="flex: 1;">Cancel</button>
        <button class="primary" @click="create" style="flex: 1;" :disabled="!selectedUser">Create</button>
      </template>
    </Modal>

    <Modal v-if="createdKey" title="Access Key Created" @close="createdKey = null">
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
      <template #footer>
        <button class="primary" style="width: 100%;" @click="createdKey = null">I've saved them</button>
      </template>
    </Modal>
  </div>
</template>
