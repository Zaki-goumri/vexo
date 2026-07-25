<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listPolicies, createPolicy, updatePolicy, deletePolicy, type Policy, type PolicyStatement } from '../api'
import Icon from '../components/Icon.vue'
import Modal from '../components/Modal.vue'
import Badge from '../components/Badge.vue'
import EmptyState from '../components/EmptyState.vue'

interface StatementDraft {
  Effect: 'Allow' | 'Deny'
  actionText: string
  resourceText: string
}

const COMMON_ACTIONS = ['s3:GetObject', 's3:PutObject', 's3:DeleteObject', 's3:ListBucket', 's3:CreateBucket', 's3:DeleteBucket', 's3:GetBucketLocation', '*']

const policies = ref<Policy[]>([])
const selected = ref<Policy | null>(null)
const error = ref('')

const showEditor = ref(false)
const editorMode = ref<'create' | 'edit'>('create')
const editorName = ref('')
const editorStatements = ref<StatementDraft[]>([])
const useRawJson = ref(false)
const rawJsonText = ref('')
const editorError = ref('')

async function load() {
  try {
    policies.value = await listPolicies()
    if (selected.value) {
      selected.value = policies.value.find((p) => p.Name === selected.value?.Name) || null
    }
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(load)

function draftsToStatements(drafts: StatementDraft[]): PolicyStatement[] {
  return drafts.map((d) => ({
    Effect: d.Effect,
    Action: d.actionText.split('\n').map((s) => s.trim()).filter(Boolean),
    Resource: d.resourceText.split('\n').map((s) => s.trim()).filter(Boolean),
  }))
}

function statementsToDrafts(statements: PolicyStatement[]): StatementDraft[] {
  return statements.map((s) => ({
    Effect: s.Effect === 'Deny' ? 'Deny' : 'Allow',
    actionText: s.Action.join('\n'),
    resourceText: s.Resource.join('\n'),
  }))
}

function openCreate() {
  editorMode.value = 'create'
  editorName.value = ''
  editorStatements.value = [{ Effect: 'Allow', actionText: '', resourceText: '' }]
  useRawJson.value = false
  rawJsonText.value = ''
  editorError.value = ''
  showEditor.value = true
}

function openEdit(p: Policy) {
  editorMode.value = 'edit'
  editorName.value = p.Name
  editorStatements.value = statementsToDrafts(p.Statement)
  useRawJson.value = false
  rawJsonText.value = ''
  editorError.value = ''
  showEditor.value = true
}

function addStatement() {
  editorStatements.value.push({ Effect: 'Allow', actionText: '', resourceText: '' })
}

function removeStatement(i: number) {
  editorStatements.value.splice(i, 1)
}

function toggleRawJson() {
  editorError.value = ''
  if (!useRawJson.value) {
    rawJsonText.value = JSON.stringify(draftsToStatements(editorStatements.value), null, 2)
    useRawJson.value = true
    return
  }
  try {
    const parsed = JSON.parse(rawJsonText.value)
    if (!Array.isArray(parsed)) throw new Error('must be a JSON array of statements')
    editorStatements.value = statementsToDrafts(parsed)
    useRawJson.value = false
  } catch (e: any) {
    editorError.value = `Invalid JSON: ${e.message}`
  }
}

async function submitEditor() {
  editorError.value = ''
  let statement: PolicyStatement[]
  if (useRawJson.value) {
    try {
      statement = JSON.parse(rawJsonText.value)
      if (!Array.isArray(statement) || statement.length === 0) throw new Error('must be a non-empty JSON array')
    } catch (e: any) {
      editorError.value = `Invalid JSON: ${e.message}`
      return
    }
  } else {
    statement = draftsToStatements(editorStatements.value)
  }

  try {
    if (editorMode.value === 'create') {
      await createPolicy(editorName.value, statement)
    } else {
      await updatePolicy(editorName.value, statement)
    }
    showEditor.value = false
    await load()
  } catch (e: any) {
    editorError.value = e.message
  }
}

async function removePolicy(name: string) {
  if (!confirm(`Delete policy "${name}"?`)) return
  try {
    await deletePolicy(name)
    if (selected.value?.Name === name) selected.value = null
    await load()
  } catch (e: any) {
    error.value = e.message
  }
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
      <h2 style="font-size: 20px;">Policies</h2>
      <button class="primary" @click="openCreate"><Icon name="shield" :size="14" /> Create Policy +</button>
    </div>
    <div v-if="error" style="color: var(--danger); margin-bottom: 16px;">{{ error }}</div>
    <div style="display: flex; gap: 16px; align-items: flex-start;">
      <div class="card" style="width: 280px; max-height: 60vh; overflow-y: auto; padding: 0; flex-shrink: 0;">
        <table v-if="policies.length > 0">
          <tbody>
            <tr v-for="p in policies" :key="p.Name" @click="selected = p"
              :style="{ cursor: 'pointer', background: selected?.Name === p.Name ? 'var(--surface2)' : 'transparent' }">
              <td style="display: flex; align-items: center; gap: 10px;"><Icon name="shield" :size="14" /> {{ p.Name }}</td>
            </tr>
          </tbody>
        </table>
        <EmptyState v-else text="No policies" />
      </div>
      <div v-if="selected" class="card" style="flex: 1;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
          <h3>{{ selected.Name }}</h3>
          <div style="display: flex; gap: 8px;">
            <button class="secondary" @click="openEdit(selected)"><Icon name="rename" :size="13" /> Edit</button>
            <button class="danger" @click="removePolicy(selected.Name)" :disabled="selected.Name === 'root'"><Icon name="trash" :size="13" /></button>
          </div>
        </div>
        <p style="color: var(--text2); margin-bottom: 16px; font-size: 12px;">Version: {{ selected.Version }}</p>
        <div v-for="(stmt, i) in selected.Statement" :key="i" style="margin-bottom: 16px; padding: 12px; background: var(--surface2); border-radius: var(--radius);">
          <div style="margin-bottom: 8px;">
            <Badge :tone="stmt.Effect === 'Allow' ? 'success' : 'danger'">{{ stmt.Effect }}</Badge>
          </div>
          <div style="margin-bottom: 6px;">
            <span style="color: var(--text2); font-size: 12px;">Actions:</span>
            <code style="margin-left: 8px;">{{ stmt.Action.join(', ') }}</code>
          </div>
          <div>
            <span style="color: var(--text2); font-size: 12px;">Resources:</span>
            <code style="margin-left: 8px; word-break: break-all;">{{ stmt.Resource.join(', ') }}</code>
          </div>
        </div>
      </div>
      <div v-else style="flex: 1; display: flex; align-items: center; justify-content: center; color: var(--text2); padding: 40px;">Select a policy to view details</div>
    </div>

    <Modal v-if="showEditor" :title="editorMode === 'create' ? 'Create Policy' : `Edit Policy: ${editorName}`" :width="520" @close="showEditor = false">
      <div v-if="editorMode === 'create'" style="margin-bottom: 16px;">
        <label style="display: block; margin-bottom: 6px; color: var(--text2); font-size: 12px;">Name</label>
        <input v-model="editorName" placeholder="policy-name" />
      </div>

      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
        <label style="color: var(--text2); font-size: 12px;">Statements</label>
        <a @click="toggleRawJson" style="cursor: pointer; font-size: 12px;">{{ useRawJson ? 'Use form' : 'Edit raw JSON' }}</a>
      </div>

      <div v-if="useRawJson" style="margin-bottom: 16px;">
        <textarea v-model="rawJsonText" rows="12" style="font-family: monospace; font-size: 12px;"></textarea>
      </div>

      <div v-else style="margin-bottom: 8px; max-height: 340px; overflow-y: auto;">
        <div v-for="(stmt, i) in editorStatements" :key="i" style="margin-bottom: 12px; padding: 12px; background: var(--surface2); border-radius: var(--radius);">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
            <select v-model="stmt.Effect" style="width: 120px;">
              <option value="Allow">Allow</option>
              <option value="Deny">Deny</option>
            </select>
            <button class="danger" @click="removeStatement(i)" :disabled="editorStatements.length === 1"><Icon name="trash" :size="12" /></button>
          </div>
          <div style="margin-bottom: 8px;">
            <label style="display: block; margin-bottom: 4px; color: var(--text2); font-size: 11px;">Actions (one per line — e.g. s3:GetObject, or *)</label>
            <textarea v-model="stmt.actionText" rows="2" style="font-family: monospace; font-size: 12px;" :placeholder="COMMON_ACTIONS.slice(0, 3).join('\n')"></textarea>
          </div>
          <div>
            <label style="display: block; margin-bottom: 4px; color: var(--text2); font-size: 11px;">Resources (one per line — e.g. arn:aws:s3:::mybucket/*)</label>
            <textarea v-model="stmt.resourceText" rows="2" style="font-family: monospace; font-size: 12px;" placeholder="arn:aws:s3:::mybucket/*"></textarea>
          </div>
        </div>
        <button class="secondary" @click="addStatement" style="width: 100%;">+ Add statement</button>
      </div>

      <div v-if="editorError" style="color: var(--danger); font-size: 12px; margin-bottom: 12px;">{{ editorError }}</div>

      <template #footer>
        <button class="secondary" @click="showEditor = false" style="flex: 1;">Cancel</button>
        <button class="primary" @click="submitEditor" style="flex: 1;">{{ editorMode === 'create' ? 'Create' : 'Save' }}</button>
      </template>
    </Modal>
  </div>
</template>
