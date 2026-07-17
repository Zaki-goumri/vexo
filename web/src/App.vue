<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api } from './api'

const authed = ref(false)

onMounted(async () => {
  try {
    const data = await api.get<{authenticated: boolean; username?: string}>('/session')
    authed.value = data.authenticated
  } catch {
    authed.value = false
  }
})
</script>

<template>
  <router-view />
</template>