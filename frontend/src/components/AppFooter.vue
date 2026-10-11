<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useInit } from '../composables/useInit.js'
import { canAccessControlPanel } from '../composables/rbacAccess.js'

const init = useInit()
const showFooter = computed(() => init.features.showFooter)
const showVersion = computed(() => init.features.showVersionNumber && init.currentVersion)
const showSystem = computed(() => canAccessControlPanel())
</script>

<template>
  <footer v-if="showFooter">
    <div>{{ init.siteTitle }}</div>
    <div>
      <span v-if="showVersion">{{ init.currentVersion }}</span>
      <RouterLink v-if="showSystem" :to="{ name: 'control-panel' }">System</RouterLink>
      <a href="https://github.com/jamesread/RepoManager" target="_blank" rel="noopener">GitHub</a>
    </div>
  </footer>
</template>
