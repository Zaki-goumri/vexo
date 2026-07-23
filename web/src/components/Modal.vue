<script setup lang="ts">
defineProps<{ title: string; width?: number }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="card" :style="{ width: (width || 420) + 'px' }">
      <h3 style="margin-bottom: 16px;">{{ title }}</h3>
      <slot />
      <div v-if="$slots.footer" style="display: flex; gap: 8px; margin-top: 16px;">
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>
