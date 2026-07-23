<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listUsers, createUser, deleteUser, type User } from '../api'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import Badge from '../components/Badge.vue'
import EmptyState from '../components/EmptyState.vue'

const users = ref<User[]>([])
const error = ref('')
const showCreate = ref(false)
const newName = ref('')

async function load() {
  try {
    users.value = await listUsers()
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

async function create() {
  error.value = ''
  try {
    await createUser(newName.value)
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
    await deleteUser(name)
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
      <button class="primary" @click="showCreate = true">Create User +</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div class="card" style="padding: 0;">
      <table v-if="users.length > 0">
        <thead>
          <tr><th>Username</th><th>Status</th><th>Policies</th><th>Groups</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.username">
            <td style="display: flex; align-items: center; gap: 10px;"><Icon name="user" :size="15" /> {{ u.username }}</td>
            <td><Badge :tone="u.status === 'active' ? 'success' : 'danger'">{{ u.status }}</Badge></td>
            <td style="color: var(--text2);">{{ u.policies?.join(', ') || '—' }}</td>
            <td style="color: var(--text2);">{{ u.groups?.join(', ') || '—' }}</td>
            <td style="text-align: right;"><button class="danger" @click="remove(u.username)"><Icon name="trash" :size="14" /></button></td>
          </tr>
        </tbody>
      </table>
      <EmptyState v-else text="No users yet" />
    </div>

    <Modal v-if="showCreate" title="Create User" @close="showCreate = false">
      <input v-model="newName" placeholder="username" @keyup.enter="create" style="margin-bottom: 16px;" />
      <template #footer>
        <button class="secondary" @click="showCreate = false" style="flex: 1;">Cancel</button>
        <button class="primary" @click="create" style="flex: 1;">Create</button>
      </template>
    </Modal>
  </div>
</template>
