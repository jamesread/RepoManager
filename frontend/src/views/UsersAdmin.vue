<script setup>
import { onMounted, ref } from 'vue'
import { client } from '../composables/client.js'

const users = ref([])

onMounted(async () => {
  const res = await client.listUsers({})
  users.value = res.users ?? []
})
</script>

<template>
  <table>
    <thead>
      <tr><th>ID</th><th>Username</th><th>Created by</th></tr>
    </thead>
    <tbody>
      <tr v-for="u in users" :key="u.id">
        <td>{{ u.id }}</td>
        <td>{{ u.username }}</td>
        <td>{{ u.createdBy }}</td>
      </tr>
    </tbody>
  </table>
</template>
