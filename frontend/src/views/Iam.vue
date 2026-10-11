<script setup>
import { RouterLink } from 'vue-router'
import { UserGroupIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
</script>

<template>
  <Section title="Identity &amp; Access" :icon="UserGroupIcon">
    <nav>
      <RouterLink :to="{ name: 'iam-users' }">Users</RouterLink>
      <RouterLink :to="{ name: 'my-permissions' }">My permissions</RouterLink>
    </nav>
    <RouterView />
  </Section>
</template>
