<script setup>
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { SecurityValidationIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import { client } from '../composables/client.js'

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const saving = ref(false)
const error = ref('')
const success = ref('')

async function save() {
  error.value = ''
  success.value = ''
  if (newPassword.value !== confirmPassword.value) {
    error.value = 'New passwords do not match'
    return
  }
  if (newPassword.value.length < 8) {
    error.value = 'New password must be at least 8 characters'
    return
  }
  saving.value = true
  try {
    const res = await client.changePassword({
      currentPassword: currentPassword.value,
      newPassword: newPassword.value,
    })
    if (!res.success) {
      error.value = res.message || 'Password change failed'
      return
    }
    success.value = res.message || 'Password updated'
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Section title="Change Password" :icon="SecurityValidationIcon">
    <template #toolbar>
      <RouterLink :to="{ name: 'userControlPanel' }" class="button neutral">Back</RouterLink>
    </template>

    <p v-if="error" class="form-error padding">{{ error }}</p>
    <p v-if="success" class="padding">{{ success }}</p>

    <FormLayout class="padding" @submit.prevent="save">
      <FormField label="Current password" for="current-password" :disabled="saving">
        <input
          id="current-password"
          v-model="currentPassword"
          type="password"
          autocomplete="current-password"
          required
          :disabled="saving"
        />
      </FormField>
      <FormField label="New password" for="new-password" :disabled="saving"
        description="At least 8 characters.">
        <input
          id="new-password"
          v-model="newPassword"
          type="password"
          autocomplete="new-password"
          required
          minlength="8"
          :disabled="saving"
        />
      </FormField>
      <FormField label="Confirm new password" for="confirm-password" :disabled="saving">
        <input
          id="confirm-password"
          v-model="confirmPassword"
          type="password"
          autocomplete="new-password"
          required
          minlength="8"
          :disabled="saving"
        />
      </FormField>
      <template #actions>
        <button type="submit" class="good" :disabled="saving">
          {{ saving ? 'Saving…' : 'Change password' }}
        </button>
      </template>
    </FormLayout>
  </Section>
</template>
