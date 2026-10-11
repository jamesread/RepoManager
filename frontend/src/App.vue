<script setup>
import { computed, onMounted } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import AppLayout from './components/AppLayout.vue'
import AppFooter from './components/AppFooter.vue'
import { loadInit, useInit } from './composables/useInit.js'

const init = useInit()
const route = useRoute()

onMounted(() => {
  loadInit()
})

const isLoggedIn = computed(() => !!init.user || route.name === 'login')
const showShell = computed(() => init.ready && (isLoggedIn.value || route.name === 'login'))
</script>

<template>
  <div v-if="!init.ready" class="loading">Loading…</div>
  <template v-else-if="showShell">
    <AppLayout v-if="route.name !== 'login'">
      <RouterView />
      <AppFooter />
    </AppLayout>
    <RouterView v-else />
  </template>
  <div v-else class="loading">
    <RouterView />
  </div>
</template>
