<script setup lang="ts">
const props = defineProps<{ tone: 'success' | 'danger' | 'neutral' }>()
</script>

<template>
  <span class="badge" :class="`badge-${props.tone}`"><slot /></span>
</template>
