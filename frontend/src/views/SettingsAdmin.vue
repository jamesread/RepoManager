<script setup>
import { onMounted, ref } from 'vue'
import { Settings01Icon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import { client } from '../composables/client.js'
import { loadInit } from '../composables/useInit.js'

const cvars = ref([])
const saving = ref(null)

async function load() {
  const res = await client.listCvars({})
  cvars.value = res.cvars ?? []
}

async function save(row) {
  saving.value = row.key
  try {
    await client.updateCvar({
      key: row.key,
      valueInt: row.valueInt ?? 0,
      valueString: row.valueString ?? '',
    })
    await loadInit()
    await load()
  } finally {
    saving.value = null
  }
}

onMounted(load)
</script>

<template>
  <Section title="Settings" :icon="Settings01Icon">
    <form v-for="row in cvars" :key="row.key" @submit.prevent="save(row)">
      <label>
        <strong>{{ row.title || row.key }}</strong>
        <p>{{ row.description }}</p>
        <input
          v-if="row.mainType === 'string'"
          v-model="row.valueString"
          type="text"
        />
        <select
          v-else-if="row.mainType === 'bool'"
          v-model.number="row.valueInt"
        >
          <option :value="1">Enabled</option>
          <option :value="0">Disabled</option>
        </select>
        <input v-else v-model.number="row.valueInt" type="number" />
      </label>
      <button type="submit" :disabled="saving === row.key">Save</button>
    </form>
  </Section>
</template>
