<script setup lang="ts">
import AppSidebar from '../components/AppSidebar.vue'
</script>

<template>
  <div style="display: flex; height: 100vh;">
    <AppSidebar />
    <main style="flex: 1; padding: 32px; overflow-y: auto; background: var(--bg);">
      <router-view />
    </main>
  </div>
</template>
