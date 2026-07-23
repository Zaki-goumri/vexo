<script setup lang="ts">
import { icons } from './icons'

const props = withDefaults(defineProps<{ name: keyof typeof icons; size?: number }>(), {
  size: 16,
})
</script>

<template>
  <svg
    :width="props.size"
    :height="props.size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
  >
    <path :d="icons[props.name]" />
  </svg>
</template>
