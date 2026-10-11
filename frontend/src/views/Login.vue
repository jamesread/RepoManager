<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Login01Icon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import LoginForm from 'picocrank/vue/components/Login.vue'
import { client } from '../composables/client.js'
import { loadInit, setUser } from '../composables/useInit.js'

const router = useRouter()
const route = useRoute()
const loginRef = ref(null)

async function onLocalLogin({ username, password }) {
  try {
    const res = await client.login({ username, password })
    if (!res.success) {
      loginRef.value?.setLocalLoginError(res.message || 'Login failed')
      return
    }
    if (res.token) {
      localStorage.setItem('repomanager-token', res.token)
    }
    await loadInit()
    setUser({ username: res.token ? username : username })
    loginRef.value?.resetLocalForm()
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : ''
    if (redirect.startsWith('/')) {
      await router.push(redirect)
    } else {
      await router.push({ name: 'projects' })
    }
  } catch (e) {
    loginRef.value?.setLocalLoginError(e.message || 'Login failed')
  }
}
</script>

<template>
  <Section title="Sign in" :icon="Login01Icon">
    <LoginForm ref="loginRef" local-only @local-login="onLocalLogin" />
  </Section>
</template>
