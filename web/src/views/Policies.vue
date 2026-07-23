<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listPolicies, type Policy } from '../api'
import Icon from '../components/Icon.vue'
import Badge from '../components/Badge.vue'
import EmptyState from '../components/EmptyState.vue'

const policies = ref<Policy[]>([])
const selected = ref<Policy | null>(null)

async function load() {
  policies.value = await listPolicies()
}

onMounted(load)
</script>

<template>
  <div>
    <div style="margin-bottom: 24px;">
      <h2 style="font-size: 20px; margin-bottom: 4px;">Policies</h2>
      <p style="color: var(--text2); font-size: 12px;">Read-only — policy creation/editing isn't available yet.</p>
    </div>
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
        <h3 style="margin-bottom: 12px;">{{ selected.Name }}</h3>
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
  </div>
</template>
