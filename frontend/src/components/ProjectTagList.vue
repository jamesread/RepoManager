<script setup>
import { computed } from 'vue'
import ProjectTag from './ProjectTag.vue'
import { displayProjectTags, maxVisibleProjectTags } from '../composables/api'

const props = defineProps({
  tags: {
    type: [String, Array],
    default: () => [],
  },
  tagColors: {
    type: Object,
    default: () => ({}),
  },
  tagOrder: {
    type: Array,
    default: () => [],
  },
  maxVisible: {
    type: Number,
    default: maxVisibleProjectTags,
  },
})

const display = computed(() =>
  displayProjectTags(props.tags, {
    maxVisible: props.maxVisible,
    tagOrder: props.tagOrder,
  }),
)
</script>

<template>
  <span class="tag-list">
    <ProjectTag
      v-for="tag in display.visible"
      :key="tag"
      :tag="tag"
      :tag-colors="tagColors"
    />
    <span v-if="display.hiddenCount" class="tag tag-overflow" :title="`${display.hiddenCount} more tags`">
      +{{ display.hiddenCount }}
    </span>
  </span>
</template>
