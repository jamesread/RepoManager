<script setup>
import { computed } from 'vue'
import { isGitHubTag, tagChipStyle } from '../composables/api'

const props = defineProps({
  tag: {
    type: String,
    required: true,
  },
  tagColors: {
    type: Object,
    default: () => ({}),
  },
  toggle: {
    type: Boolean,
    default: false,
  },
  pressed: {
    type: Boolean,
    default: false,
  },
  mixed: {
    type: Boolean,
    default: false,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['click'])

const github = computed(() => isGitHubTag(props.tag))
const chipStyle = computed(() => (github.value ? null : tagChipStyle(props.tagColors, props.tag)))
</script>

<template>
  <component
    :is="toggle ? 'button' : 'span'"
    class="tag"
    :class="{
      'tag-github': github,
      'tag-toggle': toggle,
      'tag-toggle-on': toggle && pressed,
      'tag-toggle-mixed': toggle && mixed,
    }"
    :style="chipStyle"
    :type="toggle ? 'button' : undefined"
    :aria-pressed="toggle ? pressed : undefined"
    :aria-label="github ? 'GitHub' : undefined"
    :title="github ? 'GitHub' : tag"
    :disabled="toggle ? disabled : undefined"
    @click="toggle ? $emit('click', $event) : undefined"
  >
    <svg
      v-if="github"
      class="github-mark"
      viewBox="0 0 98 96"
      aria-hidden="true"
      focusable="false"
    >
      <path
        fill="currentColor"
        fill-rule="evenodd"
        clip-rule="evenodd"
        d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.465-5.867-16.465-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.067-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.067-13.202-1.426-3.492-3.574-10.839 1.267-13.202 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0112.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 4.841 2.363 2.693 9.71 1.267 13.202 3.127 3.424 5.067 7.823 5.067 13.202 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"
      />
    </svg>
    <template v-else>{{ tag }}</template>
  </component>
</template>
