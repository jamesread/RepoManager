<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Database01Icon, Settings01Icon, UserGroupIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import { canAccessDatabase, canAccessIam, canAccessSettings } from '../composables/rbacAccess'

const tiles = computed(() => {
  const items = []
  if (canAccessDatabase()) {
    items.push({
      title: 'Database',
      description: 'Connection status and schema migrations',
      icon: Database01Icon,
      to: { name: 'system-database' },
    })
  }
  if (canAccessSettings()) {
    items.push({
      title: 'Settings',
      description: 'System configuration variables',
      icon: Settings01Icon,
      to: { name: 'settings' },
    })
  }
  if (canAccessIam()) {
    items.push({
      title: 'Identity & Access',
      description: 'Users, groups, roles, and permissions',
      icon: UserGroupIcon,
      to: { name: 'iam-users' },
    })
  }
  return items
})
</script>

<template>
  <Section title="System Control Panel" subtitle="System and app settings">
    <Navigation>
      <NavigationGrid :items="tiles" />
    </Navigation>
    <p v-if="tiles.length === 0" class="subtle padding">No system pages are available for your account.</p>
    <p class="padding">
      <RouterLink :to="{ name: 'projects' }">Back to projects</RouterLink>
    </p>
  </Section>
</template>
