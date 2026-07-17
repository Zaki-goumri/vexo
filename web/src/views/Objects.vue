<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'

const route = useRoute()
const router = useRouter()
const bucketName = route.params.name as string

interface ObjectMeta {
  ID: string
  Bucket: string
  Key: string
  Size: number
  ETag: string
  Tier: string
  CreatedAt: string
  LastAccessedAt: string
  AccessCount: number
}
const objects = ref<ObjectMeta[]>([])
const error = ref('')
const showUpload = ref(false)
const uploadKey = ref('')
const uploadFile = ref<File | null>(null)
const prefix = ref('')

async function load() {
  try {
    objects.value = await api.get<ObjectMeta[]>(`/buckets/${bucketName}/objects?prefix=${prefix.value}`)
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function onFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files[0]) {
    uploadFile.value = target.files[0]
    if (!uploadKey.value) uploadKey.value = target.files[0].name
  }
}

async function upload() {
  if (!uploadFile.value) return
  error.value = ''
  try {
    const resp = await api.raw('PUT', `/buckets/${bucketName}/objects/${uploadKey.value}`, uploadFile.value)
    if (!resp.ok) throw new Error('Upload failed')
    showUpload.value = false
    uploadKey.value = ''
    uploadFile.value = null
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}

async function download(key: string) {
  const resp = await api.raw('GET', `/buckets/${bucketName}/objects/${key}`)
  if (!resp.ok) return
  const blob = await resp.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = key.split('/').pop() || 'download'
  a.click()
  URL.revokeObjectURL(url)
}

async function remove(key: string) {
  if (!confirm(`Delete "${key}"?`)) return
  try {
    const encodedKey = encodeURIComponent(key).replace(/%2F/g, '/')
    await api.del(`/buckets/${bucketName}/objects/${encodedKey}`)
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}
</script>

<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 24px; gap: 12px;">
      <a @click="router.push('/buckets')" style="cursor: pointer; color: var(--text2);">← Buckets</a>
      <h2 style="font-size: 20px;">/{{ bucketName }}</h2>
    </div>
    <div style="display: flex; justify-content: space-between; margin-bottom: 16px; gap: 8px;">
      <input v-model="prefix" placeholder="Filter by prefix..." @keyup.enter="load" style="max-width: 300px;" />
      <button class="primary" @click="showUpload = true">Upload</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div class="card">
      <table v-if="objects.length > 0">
        <thead>
          <tr><th>Key</th><th>Size</th><th>Tier</th><th>ETag</th><th>Uploaded</th><th>Actions</th></tr>
        </thead>
        <tbody>
          <tr v-for="obj in objects" :key="obj.ID">
            <td>{{ obj.Key }}</td>
            <td>{{ formatSize(obj.Size) }}</td>
            <td><span style="padding: 2px 8px; border-radius: 4px; font-size: 11px; background: var(--surface2);">{{ obj.Tier }}</span></td>
            <td style="color: var(--text2); font-family: monospace; font-size: 12px;">{{ obj.ETag.substring(0, 12) }}...</td>
            <td style="color: var(--text2);">{{ new Date(obj.CreatedAt).toLocaleDateString() }}</td>
            <td style="display: flex; gap: 4px;">
              <button class="secondary" @click="download(obj.Key)">Download</button>
              <button class="danger" @click="remove(obj.Key)">Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-else style="color: var(--text2); text-align: center; padding: 24px;">No objects in this bucket</div>
    </div>

    <div v-if="showUpload" class="modal-overlay" @click.self="showUpload = false">
      <div class="card" style="width: 460px;">
        <h3 style="margin-bottom: 16px;">Upload Object</h3>
        <div style="margin-bottom: 12px;">
          <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">File</label>
          <input type="file" @change="onFileChange" style="padding: 6px;" />
        </div>
        <div style="margin-bottom: 16px;">
          <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Key (path in bucket)</label>
          <input v-model="uploadKey" placeholder="path/to/object.jpg" />
        </div>
        <div style="display: flex; gap: 8px;">
          <button class="secondary" @click="showUpload = false" style="flex: 1;">Cancel</button>
          <button class="primary" @click="upload" style="flex: 1;">Upload</button>
        </div>
      </div>
    </div>
  </div>
</template>