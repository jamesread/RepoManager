<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  src: {
    type: String,
    default: '',
  },
  size: {
    type: String,
    default: 'sm',
  },
})

const failed = ref(false)

watch(() => props.src, () => {
  failed.value = false
})
</script>

<template>
  <span class="project-icon" :class="size === 'lg' ? 'project-icon-lg' : ''" aria-hidden="true">
    <img
      v-if="src && !failed"
      :src="src"
      alt=""
      @error="failed = true"
    >
    <svg
      v-else
      class="project-icon-placeholder"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.75"
      stroke-linejoin="round"
      stroke-linecap="round"
    >
      <rect x="4" y="4" width="16" height="16" rx="3" />
      <circle cx="9" cy="9" r="1.4" fill="currentColor" stroke="none" />
      <path d="M4.5 16.5 9 12l3 3 3.5-4 4 4.5" />
    </svg>
  </span>
</template>

<style scoped>
.project-icon {
  width: 1.25rem;
  height: 1.25rem;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.project-icon img,
.project-icon svg {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.project-icon-lg {
  width: 4.5rem;
  height: 4.5rem;
}

.project-icon-placeholder {
  color: var(--text-color);
  opacity: 0.4;
}
</style>
