<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { listBuckets, createBucket, deleteBucket, type Bucket } from '../api'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import EmptyState from '../components/EmptyState.vue'

const router = useRouter()
const buckets = ref<Bucket[]>([])
const error = ref('')
const showCreate = ref(false)
const newName = ref('')

async function load() {
  try {
    buckets.value = await listBuckets()
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

async function create() {
  error.value = ''
  try {
    await createBucket(newName.value)
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
    await deleteBucket(name)
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
      <button class="primary" @click="showCreate = true">Create Bucket +</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div class="card" style="padding: 0;">
      <table v-if="buckets.length > 0">
        <thead>
          <tr><th>Name</th><th>Created</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="b in buckets" :key="b.Name">
            <td>
              <a @click="router.push(`/buckets/${b.Name}`)" style="cursor: pointer; display: flex; align-items: center; gap: 10px; color: var(--text);">
                <Icon name="bucket" :size="16" />
                {{ b.Name }}
              </a>
            </td>
            <td style="color: var(--text2);">{{ new Date(b.CreatedAt).toLocaleString() }}</td>
            <td style="display: flex; gap: 8px; justify-content: flex-end;">
              <button class="secondary" @click="router.push(`/buckets/${b.Name}`)">Browse</button>
              <button class="danger" @click="remove(b.Name)"><Icon name="trash" :size="14" /></button>
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState v-else text="No buckets yet" />
    </div>

    <Modal v-if="showCreate" title="Create Bucket" @close="showCreate = false">
      <input v-model="newName" placeholder="bucket-name" @keyup.enter="create" style="margin-bottom: 16px;" />
      <template #footer>
        <button class="secondary" @click="showCreate = false" style="flex: 1;">Cancel</button>
        <button class="primary" @click="create" style="flex: 1;">Create</button>
      </template>
    </Modal>
  </div>
</template>
