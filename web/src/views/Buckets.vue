<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'

const router = useRouter()
interface Bucket {
  Name: string
  CreatedAt: string
  Versioning: boolean
}
const buckets = ref<Bucket[]>([])
const error = ref('')
const showCreate = ref(false)
const newName = ref('')

async function load() {
  try {
    buckets.value = await api.get<Bucket[]>('/buckets')
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

async function create() {
  error.value = ''
  try {
    await api.post('/buckets', { name: newName.value })
    showCreate.value = false
    newName.value = ''
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}

async function remove(name: string) {
  if (!confirm(`Delete bucket "${name}"?`)) return
  try {
    await api.del(`/buckets/${name}`)
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
      <h2 style="font-size: 20px;">Buckets</h2>
      <button class="primary" @click="showCreate = true">Create Bucket</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div class="card">
      <table v-if="buckets.length > 0">
        <thead>
          <tr><th>Name</th><th>Created</th><th>Actions</th></tr>
        </thead>
        <tbody>
          <tr v-for="b in buckets" :key="b.Name">
            <td>
              <a @click="router.push(`/buckets/${b.Name}`)" style="cursor: pointer;">{{ b.Name }}</a>
            </td>
            <td style="color: var(--text2);">{{ new Date(b.CreatedAt).toLocaleString() }}</td>
            <td><button class="danger" @click="remove(b.Name)">Delete</button></td>
          </tr>
        </tbody>
      </table>
      <div v-else style="color: var(--text2); text-align: center; padding: 24px;">No buckets yet</div>
    </div>

    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="card" style="width: 400px;">
        <h3 style="margin-bottom: 16px;">Create Bucket</h3>
        <input v-model="newName" placeholder="bucket-name" @keyup.enter="create" style="margin-bottom: 16px;" />
        <div style="display: flex; gap: 8px;">
          <button class="secondary" @click="showCreate = false" style="flex: 1;">Cancel</button>
          <button class="primary" @click="create" style="flex: 1;">Create</button>
        </div>
      </div>
    </div>
  </div>
</template>