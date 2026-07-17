<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api } from '../api'

interface Policy {
  Version: string
  Name: string
  Statement: Array<{
    Effect: string
    Action: string[]
    Resource: string[]
  }>
}
const policies = ref<Policy[]>([])
const selected = ref<Policy | null>(null)

async function load() {
  policies.value = await api.get<Policy[]>('/policies')
}

onMounted(load)
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
      <h2 style="font-size: 20px;">Policies</h2>
    </div>
    <div style="display: flex; gap: 16px;">
      <div class="card" style="width: 280px; max-height: 60vh; overflow-y: auto;">
        <table v-if="policies.length > 0">
          <tbody>
            <tr v-for="p in policies" :key="p.Name" @click="selected = p"
              :style="{ cursor: 'pointer', background: selected?.Name === p.Name ? 'var(--surface2)' : 'transparent' }">
              <td>{{ p.Name }}</td>
            </tr>
          </tbody>
        </table>
        <div v-else style="color: var(--text2); text-align: center; padding: 16px;">No policies</div>
      </div>
      <div v-if="selected" class="card" style="flex: 1;">
        <h3 style="margin-bottom: 12px;">{{ selected.Name }}</h3>
        <p style="color: var(--text2); margin-bottom: 16px; font-size: 12px;">Version: {{ selected.Version }}</p>
        <div v-for="(stmt, i) in selected.Statement" :key="i" style="margin-bottom: 16px; padding: 12px; background: var(--surface2); border-radius: var(--radius);">
          <div style="margin-bottom: 8px;">
            <span :style="{ padding: '2px 8px', borderRadius: '4px', fontSize: '11px', background: stmt.Effect === 'Allow' ? 'rgba(34,197,94,0.2)' : 'rgba(239,68,68,0.2)', color: stmt.Effect === 'Allow' ? 'var(--success)' : 'var(--danger)' }">{{ stmt.Effect }}</span>
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
      <div v-else style="flex: 1; display: flex; align-items: center; justify-content: center; color: var(--text2);">Select a policy to view details</div>
    </div>
  </div>
</template>