<script setup>
import Header from 'picocrank/vue/components/Header.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { LayoutGridIcon, RefreshIcon, TableIcon } from '@hugeicons/core-free-icons'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { refreshRoot } from '../composables/api'

const route = useRoute()
const refreshing = ref(false)

async function onRefreshRoot() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    const data = await refreshRoot()
    window.dispatchEvent(new CustomEvent('plist-projects', { detail: data }))
  } finally {
    refreshing.value = false
  }
}

const navigation = computed(() => ({
  navigationLinks: [
    {
      name: 'table',
      title: 'Table',
      type: 'route',
      icon: TableIcon,
      to: { name: 'projects', query: { view: 'table' } },
    },
    {
      name: 'big-icon',
      title: 'Big icon',
      type: 'route',
      icon: LayoutGridIcon,
      to: { name: 'projects', query: { view: 'icons' } },
    },
  ],
  isActive(link) {
    if (route.name !== 'projects') return false
    const icons = route.query.view === 'icons'
    return icons ? link.name === 'big-icon' : link.name === 'table'
  },
}))
</script>

<template>
  <Header
    title="RepoManager"
    logo-url="/logo.svg"
    :navigation="navigation"
    :top-bar-navigation="navigation"
    :sidebar-enabled="false"
    :top-bar-enabled="true"
    :theme-toggle-enabled="true"
    :show-branding="true"
  >
    <template #toolbar>
      <button
        type="button"
        class="neutral"
        title="Refresh projects"
        aria-label="Refresh projects"
        :aria-busy="refreshing"
        :disabled="refreshing"
        @click="onRefreshRoot"
      >
        <HugeiconsIcon :icon="RefreshIcon" width="1em" height="1em" />
      </button>
    </template>
  </Header>
  <div id="layout">
    <div id="content">
      <main>
        <slot />
      </main>
    </div>
  </div>
</template>
