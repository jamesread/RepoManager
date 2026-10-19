<script setup>
import Header from 'picocrank/vue/components/Header.vue'
import { HugeiconsIcon } from '@hugeicons/vue'
import { BorderNone01Icon, LayoutGridIcon, RefreshIcon, Settings01Icon, TableIcon } from '@hugeicons/core-free-icons'
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { refreshRoot } from '../composables/api'
import { isCompactMode, toggleCompactMode } from '../composables/compactMode'
import { canAccessControlPanel } from '../composables/rbacAccess.js'
import { useInit } from '../composables/useInit.js'

const route = useRoute()
const router = useRouter()
const init = useInit()
const refreshing = ref(false)
const compact = ref(isCompactMode())

function onToggleCompact() {
  compact.value = toggleCompactMode()
}

const username = computed(() => init.user?.username ?? '')
const themeToggleEnabled = computed(
  () => !!init.user && init.features.themeColorSchemeSwitcherEnabled,
)
const showControlPanel = computed(() => !!init.user && canAccessControlPanel())

function goToUserControlPanel() {
  router.push({ name: 'userControlPanel' })
}

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
    :username="username"
    :theme-toggle-enabled="themeToggleEnabled"
    :login-route="{ name: 'login' }"
    :show-branding="true"
    @user-click="goToUserControlPanel"
  >
    <template #toolbar>
      <RouterLink
        v-if="showControlPanel"
        :to="{ name: 'control-panel' }"
        class="neutral header-control-panel-link"
        title="System Control Panel"
        aria-label="System Control Panel"
      >
        <HugeiconsIcon :icon="Settings01Icon" width="1em" height="1em" />
      </RouterLink>
      <button
        type="button"
        class="neutral"
        title="Compact mode"
        aria-label="Compact mode"
        :aria-pressed="compact"
        @click="onToggleCompact"
      >
        <HugeiconsIcon :icon="BorderNone01Icon" width="1em" height="1em" />
      </button>
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
