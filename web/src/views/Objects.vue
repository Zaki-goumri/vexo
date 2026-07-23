<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { listObjects, putObject, getObject, deleteObject, type ObjectMeta } from '../api'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import EmptyState from '../components/EmptyState.vue'

const route = useRoute()
const router = useRouter()
const bucketName = route.params.name as string

const rawObjects = ref<ObjectMeta[]>([])
const error = ref('')
const showUpload = ref(false)
const uploadKey = ref('')
const uploadFile = ref<File | null>(null)
const prefix = ref('')
const search = ref('')
const selected = ref<ObjectMeta | null>(null)

type Row = { type: 'folder'; name: string } | { type: 'file'; name: string; obj: ObjectMeta }

const breadcrumbs = computed(() => {
  const parts = prefix.value.split('/').filter(Boolean)
  return parts.map((name, i) => ({ name, prefix: parts.slice(0, i + 1).join('/') + '/' }))
})

const rows = computed<Row[]>(() => {
  const byName = new Map<string, Row>()
  for (const obj of rawObjects.value) {
    const rest = obj.key.slice(prefix.value.length)
    if (!rest) continue
    const slash = rest.indexOf('/')
    if (slash === -1) {
      byName.set(rest, { type: 'file', name: rest, obj })
    } else {
      const folder = rest.slice(0, slash + 1)
      if (!byName.has(folder)) byName.set(folder, { type: 'folder', name: folder })
    }
  }
  let list = Array.from(byName.values())
  if (search.value) {
    list = list.filter((r) => r.name.toLowerCase().includes(search.value.toLowerCase()))
  }
  return list.sort((a, b) => (a.type !== b.type ? (a.type === 'folder' ? -1 : 1) : a.name.localeCompare(b.name)))
})

async function load() {
  error.value = ''
  try {
    rawObjects.value = await listObjects(bucketName, prefix.value)
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)
watch(prefix, () => {
  selected.value = null
  load()
})

function openFolder(name: string) {
  prefix.value += name
}

function goToPrefix(p: string) {
  prefix.value = p
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function onFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files[0]) {
    uploadFile.value = target.files[0]
    if (!uploadKey.value) uploadKey.value = prefix.value + target.files[0].name
  }
}

async function upload() {
  if (!uploadFile.value) return
  error.value = ''
  try {
    const resp = await putObject(bucketName, uploadKey.value, uploadFile.value)
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
  const resp = await getObject(bucketName, key)
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
    await deleteObject(bucketName, key)
    if (selected.value?.key === key) selected.value = null
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}
</script>

<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 24px; gap: 8px; flex-wrap: wrap;">
      <a @click="router.push('/buckets')" style="cursor: pointer; color: var(--text2); display: flex; align-items: center;"><Icon name="bucket" :size="15" /></a>
      <span style="color: var(--text2);">/</span>
      <a @click="goToPrefix('')" style="cursor: pointer;" :style="{ color: prefix === '' ? 'var(--text)' : 'var(--accent)', fontWeight: prefix === '' ? 600 : 400 }">{{ bucketName }}</a>
      <template v-for="(crumb, i) in breadcrumbs" :key="crumb.prefix">
        <span style="color: var(--text2);">/</span>
        <a @click="goToPrefix(crumb.prefix)" style="cursor: pointer;" :style="{ color: i === breadcrumbs.length - 1 ? 'var(--text)' : 'var(--accent)', fontWeight: i === breadcrumbs.length - 1 ? 600 : 400 }">{{ crumb.name }}</a>
      </template>
    </div>
    <div style="display: flex; justify-content: space-between; margin-bottom: 16px; gap: 8px;">
      <div style="display: flex; align-items: center; gap: 8px; max-width: 320px; width: 100%; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 0 10px;">
        <Icon name="search" :size="14" />
        <input v-model="search" placeholder="Search this folder..." style="border: none; padding: 8px 0;" />
      </div>
      <button class="primary" @click="showUpload = true"><Icon name="upload" :size="14" /> Upload</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div style="display: flex; gap: 16px; align-items: flex-start;">
      <div class="card" style="padding: 0; flex: 1;">
        <table v-if="rows.length > 0">
          <thead>
            <tr><th>Name</th><th>Size</th><th>Tier</th><th>Uploaded</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.name"
              @click="row.type === 'folder' ? openFolder(row.name) : (selected = row.obj)"
              :style="{ cursor: 'pointer', background: row.type === 'file' && selected?.key === row.obj.key ? 'var(--surface2)' : 'transparent' }">
              <td style="display: flex; align-items: center; gap: 10px;">
                <Icon :name="row.type === 'folder' ? 'folder' : 'file'" :size="15" />
                {{ row.name }}
              </td>
              <td v-if="row.type === 'file'">{{ formatSize(row.obj.size) }}</td>
              <td v-else style="color: var(--text2);">—</td>
              <td v-if="row.type === 'file'"><span class="badge badge-neutral">{{ row.obj.tier }}</span></td>
              <td v-else></td>
              <td v-if="row.type === 'file'" style="color: var(--text2);">{{ new Date(row.obj.createdAt).toLocaleDateString() }}</td>
              <td v-else></td>
              <td v-if="row.type === 'file'" style="display: flex; gap: 4px; justify-content: flex-end;" @click.stop>
                <button class="secondary" @click="download(row.obj.key)"><Icon name="download" :size="13" /></button>
                <button class="danger" @click="remove(row.obj.key)"><Icon name="trash" :size="13" /></button>
              </td>
              <td v-else></td>
            </tr>
          </tbody>
        </table>
        <EmptyState v-else text="This folder is empty" />
      </div>

      <div v-if="selected" class="card" style="width: 300px; flex-shrink: 0;">
        <h3 style="margin-bottom: 16px; word-break: break-all; font-size: 14px;">{{ selected.key.split('/').pop() }}</h3>
        <div style="display: flex; flex-direction: column; gap: 12px; font-size: 13px;">
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">Full path</div><div style="word-break: break-all;">{{ selected.key }}</div></div>
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">Size</div><div>{{ formatSize(selected.size) }}</div></div>
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">Tier</div><span class="badge badge-neutral">{{ selected.tier }}</span></div>
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">ETag</div><div style="font-family: monospace; font-size: 11px; word-break: break-all;">{{ selected.etag }}</div></div>
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">Content type</div><div>{{ selected.contentType || '—' }}</div></div>
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">Created</div><div>{{ new Date(selected.createdAt).toLocaleString() }}</div></div>
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">Last accessed</div><div>{{ new Date(selected.lastAccessedAt).toLocaleString() }}</div></div>
          <div><div style="color: var(--text2); font-size: 11px; text-transform: uppercase; margin-bottom: 2px;">Access count</div><div>{{ selected.accessCount }}</div></div>
        </div>
      </div>
    </div>

    <Modal v-if="showUpload" title="Upload Object" @close="showUpload = false">
      <div style="margin-bottom: 12px;">
        <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">File</label>
        <input type="file" @change="onFileChange" style="padding: 6px;" />
      </div>
      <div style="margin-bottom: 16px;">
        <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Key (path in bucket)</label>
        <input v-model="uploadKey" placeholder="path/to/object.jpg" />
      </div>
      <template #footer>
        <button class="secondary" @click="showUpload = false" style="flex: 1;">Cancel</button>
        <button class="primary" @click="upload" style="flex: 1;">Upload</button>
      </template>
    </Modal>
  </div>
</template>
