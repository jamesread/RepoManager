<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { WebSecurityIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import { client } from '../composables/client.js'

const route = useRoute()
const audit = ref(null)
const loading = ref(true)
const error = ref('')
const standalone = computed(() => route.name === 'user-my-permissions')

onMounted(async () => {
  loading.value = true
  error.value = ''
  try {
    audit.value = await client.getMyPermissionsAudit({})
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <component :is="standalone ? Section : 'div'" v-bind="standalone ? {
    title: 'My Permissions',
    icon: WebSecurityIcon,
    subtitle: 'Effective access for your account',
  } : {}">
    <template v-if="standalone" #toolbar>
      <RouterLink :to="{ name: 'userControlPanel' }" class="button neutral">Back</RouterLink>
    </template>

    <p v-if="error" class="form-error padding">{{ error }}</p>
    <p v-else-if="loading" class="padding">Loading…</p>
    <div v-else-if="audit" class="padding">
      <dl class="detail-list account-info">
        <div>
          <dt>Superuser</dt>
          <dd>{{ audit.isSuperuser ? 'Yes' : 'No' }}</dd>
        </div>
        <div>
          <dt>Groups</dt>
          <dd>{{ (audit.groupNames ?? []).join(', ') || '—' }}</dd>
        </div>
        <div>
          <dt>Roles</dt>
          <dd>{{ (audit.roleNames ?? []).join(', ') || '—' }}</dd>
        </div>
      </dl>
      <ul>
        <li v-for="row in audit.permissions ?? []" :key="row.permission">
          {{ row.permission }} — {{ row.granted ? 'granted' : 'denied' }}
        </li>
      </ul>
    </div>
  </component>
</template>
