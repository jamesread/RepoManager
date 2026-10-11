<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { UserIcon, SecurityValidationIcon, WebSecurityIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import { client } from '../composables/client.js'
import { invalidateAppStatus, loadInit, useInit } from '../composables/useInit.js'

const init = useInit()
const router = useRouter()
const navRef = ref(null)
const refreshing = ref(false)
const loggingOut = ref(false)

const username = computed(() => init.user?.username ?? '')

async function refreshIdentity() {
  refreshing.value = true
  try {
    await loadInit()
  } finally {
    refreshing.value = false
  }
}

async function signOut() {
  if (loggingOut.value) return
  loggingOut.value = true
  try {
    await client.logout({})
  } catch {
    // Clear local session even if the server call fails.
  } finally {
    localStorage.removeItem('repomanager-token')
    invalidateAppStatus()
    await router.push({ name: 'login' })
    window.location.reload()
  }
}

onMounted(() => {
  const nav = navRef.value
  if (!nav) return
  nav.addCallback('Change Password', () => router.push({ name: 'change-password' }), {
    icon: SecurityValidationIcon,
    name: 'change-password',
    description: 'Change your account password',
  })
  nav.addCallback('My Permissions', () => router.push({ name: 'user-my-permissions' }), {
    icon: WebSecurityIcon,
    name: 'user-my-permissions',
    description: 'Review your group membership and effective permissions',
  })
})
</script>

<template>
  <Section title="User Control Panel" subtitle="Your account" :icon="UserIcon">
    <template #toolbar>
      <button type="button" class="neutral" :disabled="refreshing" @click="refreshIdentity">
        {{ refreshing ? 'Refreshing…' : 'Refresh' }}
      </button>
      <RouterLink :to="{ name: 'projects' }" class="button neutral">Back to projects</RouterLink>
    </template>

    <Section title="Identity" subtitle="Signed-in account">
      <dl class="detail-list padding account-info">
        <div>
          <dt>Username</dt>
          <dd>{{ username || '—' }}</dd>
        </div>
        <div v-if="init.rbacIsSuperuser">
          <dt>Access</dt>
          <dd>Superuser</dd>
        </div>
      </dl>
    </Section>

    <Section title="Quick actions" subtitle="Self-service account tools">
      <Navigation ref="navRef">
        <NavigationGrid />
      </Navigation>
    </Section>

    <Section title="Session">
      <div class="padding">
        <button type="button" class="bad" :disabled="loggingOut" @click="signOut">
          {{ loggingOut ? 'Signing out…' : 'Sign out' }}
        </button>
      </div>
    </Section>
  </Section>
</template>
