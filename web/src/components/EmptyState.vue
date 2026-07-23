<script setup lang="ts">
defineProps<{ text: string }>()
</script>

<template>
  <div style="color: var(--text2); text-align: center; padding: 32px;">{{ text }}</div>
</template>
