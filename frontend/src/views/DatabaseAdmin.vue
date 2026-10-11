<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Database01Icon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import { client } from '../composables/client.js'

const loading = ref(true)
const error = ref('')
const status = ref(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    status.value = await client.getDatabaseStatus({})
  } catch (e) {
    error.value = e.message || String(e)
    status.value = null
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <Section title="Database" :icon="Database01Icon" subtitle="Connection and schema status">
    <template #toolbar>
      <button type="button" class="neutral" :disabled="loading" @click="load">
        {{ loading ? 'Refreshing…' : 'Refresh' }}
      </button>
      <RouterLink :to="{ name: 'control-panel' }" class="button neutral">Back</RouterLink>
    </template>

    <p v-if="error" class="form-error padding">{{ error }}</p>
    <p v-else-if="loading && !status" class="scan-status padding">Loading…</p>
    <dl v-else-if="status" class="detail-list padding">
      <div>
        <dt>Driver</dt>
        <dd>{{ status.driver || '—' }}</dd>
      </div>
      <div>
        <dt>Host</dt>
        <dd>{{ status.host || '—' }}</dd>
      </div>
      <div>
        <dt>Database</dt>
        <dd>{{ status.database || '—' }}</dd>
      </div>
      <div>
        <dt>DSN</dt>
        <dd class="project-path">{{ status.dsnRedacted || '—' }}</dd>
      </div>
      <div>
        <dt>Connected</dt>
        <dd>{{ status.connected ? 'Yes' : 'No' }}</dd>
      </div>
      <div v-if="status.connectionError">
        <dt>Connection error</dt>
        <dd class="form-error">{{ status.connectionError }}</dd>
      </div>
      <div>
        <dt>Required migration</dt>
        <dd>{{ status.requiredMigration || '—' }}</dd>
      </div>
      <div>
        <dt>Latest migration</dt>
        <dd>{{ status.latestMigration || '—' }}</dd>
      </div>
      <div>
        <dt>Schema up to date</dt>
        <dd>{{ status.migrationsOk ? 'Yes' : 'No' }}</dd>
      </div>
      <div v-if="status.appliedMigrations?.length">
        <dt>Applied migrations</dt>
        <dd>
          <ul class="migration-list">
            <li v-for="id in status.appliedMigrations" :key="id">{{ id }}</li>
          </ul>
        </dd>
      </div>
    </dl>
  </Section>
</template>
