<script setup>
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import { sortRows } from 'picocrank/vue/composables/tableFilters.js'
import ProjectIcon from './ProjectIcon.vue'
import { projectIconURL, tagChipStyle } from '../composables/api'
import { applyProjectsFilterQuery, buildProjectsFilterQuery, splitCellTokens } from '../composables/projectTable'
import { loadIconViewState, saveIconViewState } from '../composables/projectView'

const TEXT_OPERATORS = [
  { value: 'contains', label: 'Contains' },
  { value: 'equals', label: 'Equals' },
  { value: 'startsWith', label: 'Starts with' },
  { value: 'empty', label: 'Is empty' },
]

const NUMBER_OPERATORS = [
  { value: 'eq', label: 'Equals' },
  { value: 'gt', label: 'Greater than' },
  { value: 'gte', label: 'Greater or equal' },
  { value: 'lt', label: 'Less than' },
  { value: 'lte', label: 'Less or equal' },
]

const props = defineProps({
  rows: {
    type: Array,
    default: () => [],
  },
  headers: {
    type: Array,
    default: () => [],
  },
  selectedKeys: {
    type: Array,
    default: () => [],
  },
  selectable: {
    type: Boolean,
    default: false,
  },
  tagColors: {
    type: Object,
    default: () => ({}),
  },
})

const emit = defineEmits(['update:selectedKeys'])

const savedIconView = loadIconViewState()
const sortBy = ref(savedIconView?.sortBy || 'name')
const sortDir = ref(savedIconView?.sortDir || 'asc')
const groupBy = ref(savedIconView?.groupBy || '')
const filterRows = ref((savedIconView?.filters || []).map((entry, index) => ({
  id: index + 1,
  key: entry.key,
})))
const drafts = ref(Object.fromEntries(
  (savedIconView?.filters || []).map((entry) => [entry.key, entry.draft]),
))
const filterColumn = ref('')
let filterSeq = filterRows.value.reduce((max, row) => Math.max(max, row.id), 0)
const knownHeaderKeys = new Set()

const sortableHeaders = computed(() =>
  props.headers.filter((header) => header.sortable && header.key !== 'actions'),
)

const filterableHeaders = computed(() =>
  props.headers.filter((header) => header.filterable !== false && header.key !== 'actions'),
)

const groupHeaders = computed(() =>
  props.headers.filter((header) => header.key !== 'name' && header.key !== 'path' && header.key !== 'actions'),
)

const unusedFilterHeaders = computed(() => {
  const used = new Set(filterRows.value.map((row) => row.key))
  return filterableHeaders.value.filter((header) => !used.has(header.key))
})

const activeSortBy = computed(() => {
  const keys = sortableHeaders.value.map((header) => header.key)
  if (keys.includes(sortBy.value)) return sortBy.value
  return keys[0] || 'name'
})

const headerByKey = computed(() => {
  const map = new Map()
  for (const header of props.headers) {
    if (header?.key) map.set(header.key, header)
  }
  return map
})

const selectedSet = computed(() => new Set(props.selectedKeys))

const visibleRows = computed(() => {
  const filtered = applyProjectsFilterQuery(
    props.rows,
    props.headers,
    buildProjectsFilterQuery(drafts.value),
  )
  return sortRows(filtered, activeSortBy.value, sortDir.value)
})

const displayGroups = computed(() => {
  const header = headerFor(groupBy.value)
  if (!header) {
    return [{ key: 'all', label: '', value: '', rows: visibleRows.value }]
  }
  const buckets = new Map()
  for (const row of visibleRows.value) {
    const tokens = groupTokens(row, header)
    const keys = tokens.length ? tokens : ['']
    for (const key of keys) {
      if (!buckets.has(key)) buckets.set(key, [])
      buckets.get(key).push(row)
    }
  }
  return [...buckets.entries()]
    .sort((a, b) => {
      if (a[0] === '') return 1
      if (b[0] === '') return -1
      return a[0].localeCompare(b[0])
    })
    .map(([value, rows]) => ({
      key: value || '__none__',
      label: value || 'None',
      value,
      rows,
    }))
})

watch(unusedFilterHeaders, (headers) => {
  if (!headers.some((header) => header.key === filterColumn.value)) {
    filterColumn.value = headers[0]?.key || ''
  }
}, { immediate: true })

function emptyDraft(header) {
  if (header?.filterType === 'select') return { type: 'select', values: [] }
  if (header?.filterType === 'number') return { type: 'number', operator: 'eq', value: '' }
  return { type: 'text', operator: 'contains', value: '' }
}

function headerKeyGone(key, alive) {
  return knownHeaderKeys.has(key) && !alive.has(key)
}

function alignDraft(key, draft) {
  const header = headerByKey.value.get(key)
  if (!header) return draft
  const fresh = emptyDraft(header)
  if (!draft || draft.type !== fresh.type) return fresh
  if (fresh.type === 'select') {
    return { type: 'select', values: Array.isArray(draft.values) ? draft.values.map(String) : [] }
  }
  if (fresh.type === 'number') {
    const allowed = new Set(NUMBER_OPERATORS.map((op) => op.value))
    return {
      type: 'number',
      operator: allowed.has(draft.operator) ? draft.operator : 'eq',
      value: draft.value == null ? '' : String(draft.value),
    }
  }
  const allowed = new Set(TEXT_OPERATORS.map((op) => op.value))
  return {
    type: 'text',
    operator: allowed.has(draft.operator) ? draft.operator : 'contains',
    value: draft.value == null ? '' : String(draft.value),
  }
}

watch(() => props.headers, (headers) => {
  const alive = new Set(headers.map((header) => header.key))
  const drop = (key) => headerKeyGone(key, alive)
  for (const header of headers) {
    if (header?.key) knownHeaderKeys.add(header.key)
  }
  if (groupBy.value && drop(groupBy.value)) groupBy.value = ''
  const nextRows = filterRows.value.filter((row) => !drop(row.key))
  if (nextRows.length !== filterRows.value.length) filterRows.value = nextRows
  let draftsChanged = false
  const next = { ...drafts.value }
  for (const key of Object.keys(next)) {
    if (drop(key)) {
      delete next[key]
      draftsChanged = true
      continue
    }
    if (!alive.has(key)) continue
    const aligned = alignDraft(key, next[key])
    if (JSON.stringify(aligned) !== JSON.stringify(next[key])) {
      next[key] = aligned
      draftsChanged = true
    }
  }
  if (draftsChanged) drafts.value = next
}, { immediate: true })

watch([sortBy, sortDir, groupBy, filterRows, drafts], () => {
  saveIconViewState({
    sortBy: sortBy.value,
    sortDir: sortDir.value,
    groupBy: groupBy.value,
    filters: filterRows.value.map((row) => ({
      key: row.key,
      draft: drafts.value[row.key],
    })),
  })
}, { deep: true })

function headerFor(key) {
  return headerByKey.value.get(key)
}

function groupTokens(row, header) {
  const value = row?.[header.key]
  if (value == null || value === '') return []
  if (header.multiValue) return splitCellTokens(value)
  return [String(value)]
}

function draftFor(key) {
  return drafts.value[key] || emptyDraft(headerFor(key))
}

function writeDraft(key, partial) {
  drafts.value = { ...drafts.value, [key]: partial }
}

function addFilter() {
  const key = filterColumn.value
  if (!key || filterRows.value.some((row) => row.key === key)) return
  filterSeq += 1
  filterRows.value = [...filterRows.value, { id: filterSeq, key }]
  writeDraft(key, emptyDraft(headerFor(key)))
}

function removeFilter(id) {
  const row = filterRows.value.find((item) => item.id === id)
  filterRows.value = filterRows.value.filter((item) => item.id !== id)
  if (!row) return
  const next = { ...drafts.value }
  delete next[row.key]
  drafts.value = next
}

function clearFilters() {
  filterRows.value = []
  drafts.value = {}
}

function setTextOperator(key, operator) {
  const current = draftFor(key)
  writeDraft(key, { ...current, type: 'text', operator })
}

function setTextValue(key, value) {
  const current = draftFor(key)
  writeDraft(key, { ...current, type: 'text', value })
}

function setNumberOperator(key, operator) {
  const current = draftFor(key)
  writeDraft(key, { ...current, type: 'number', operator })
}

function setNumberValue(key, value) {
  const current = draftFor(key)
  writeDraft(key, { ...current, type: 'number', value })
}

function setSelectValues(key, values) {
  writeDraft(key, { type: 'select', values: values || [] })
}

function toggleSortDir() {
  sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
}

function toggleSelected(id) {
  if (!props.selectable) return
  const next = new Set(props.selectedKeys)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  emit('update:selectedKeys', [...next])
}

function tileBindings(row) {
  if (props.selectable) return {}
  return {
    to: { name: 'project', query: { path: row.id } },
    title: row.path,
  }
}

function onTileClick(event, id) {
  if (!props.selectable) return
  if (event.target.closest('a, input, label, button')) return
  toggleSelected(id)
}

function tagList(value) {
  return String(value || '').split(', ').filter(Boolean)
}
</script>

<template>
  <div class="icon-view">
    <div class="icon-view-toolbar">
      <label class="icon-view-sort">
        <span>Sort</span>
        <select :value="activeSortBy" @change="sortBy = $event.target.value">
          <option v-for="header in sortableHeaders" :key="header.key" :value="header.key">
            {{ header.label }}
          </option>
        </select>
      </label>
      <button type="button" class="neutral" @click="toggleSortDir">
        {{ sortDir === 'asc' ? 'Ascending' : 'Descending' }}
      </button>
      <label class="icon-view-sort">
        <span>Group</span>
        <select v-model="groupBy">
          <option value="">None</option>
          <option v-for="header in groupHeaders" :key="header.key" :value="header.key">
            {{ header.label }}
          </option>
        </select>
      </label>
      <span class="subtle">{{ visibleRows.length }} shown</span>
    </div>

    <div class="icon-view-filters">
      <div class="icon-view-filter-add">
        <label>
          <span>Filter</span>
          <select v-model="filterColumn" :disabled="unusedFilterHeaders.length === 0">
            <option v-for="header in unusedFilterHeaders" :key="header.key" :value="header.key">
              {{ header.label }}
            </option>
          </select>
        </label>
        <button
          type="button"
          class="neutral"
          :disabled="!filterColumn"
          @click="addFilter"
        >Add filter</button>
        <button
          v-if="filterRows.length"
          type="button"
          class="neutral"
          @click="clearFilters"
        >Clear filters</button>
      </div>

      <div v-for="row in filterRows" :key="row.id" class="icon-view-filter">
        <div class="icon-view-filter-head">
          <span>{{ headerFor(row.key)?.label || row.key }}</span>
          <button type="button" class="neutral" @click="removeFilter(row.id)">Remove</button>
        </div>

        <template v-if="headerFor(row.key)?.filterType === 'select'">
          <div v-if="(headerFor(row.key).filterOptions || []).length" class="icon-view-checks">
            <CheckGroup
              :model-value="draftFor(row.key).values || []"
              :options="headerFor(row.key).filterOptions"
              :aria-label="`Filter by ${headerFor(row.key).label}`"
              @update:model-value="setSelectValues(row.key, $event)"
            />
          </div>
          <p v-else class="subtle">No values yet.</p>
        </template>

        <template v-else-if="headerFor(row.key)?.filterType === 'number'">
          <div class="icon-view-filter-fields">
            <select
              :value="draftFor(row.key).operator || 'eq'"
              @change="setNumberOperator(row.key, $event.target.value)"
            >
              <option v-for="op in NUMBER_OPERATORS" :key="op.value" :value="op.value">{{ op.label }}</option>
            </select>
            <input
              type="number"
              :value="draftFor(row.key).value"
              @input="setNumberValue(row.key, $event.target.value)"
            >
          </div>
        </template>

        <template v-else>
          <div class="icon-view-filter-fields">
            <select
              :value="draftFor(row.key).operator || 'contains'"
              @change="setTextOperator(row.key, $event.target.value)"
            >
              <option v-for="op in TEXT_OPERATORS" :key="op.value" :value="op.value">{{ op.label }}</option>
            </select>
            <input
              v-if="draftFor(row.key).operator !== 'empty'"
              type="text"
              :value="draftFor(row.key).value || ''"
              @input="setTextValue(row.key, $event.target.value)"
            >
          </div>
        </template>
      </div>
    </div>

    <p v-if="visibleRows.length === 0" class="scan-status padding">
      No projects match the current filters.
    </p>
    <template v-else>
      <div v-for="group in displayGroups" :key="group.key" class="icon-group">
        <h3 v-if="group.label" class="icon-group-title">
          <span
            v-if="groupBy === 'tags' && group.value"
            class="tag"
            :style="tagChipStyle(tagColors, group.value)"
          >{{ group.label }}</span>
          <span v-else>{{ group.label }}</span>
          <span class="subtle">{{ group.rows.length }}</span>
        </h3>
        <ul class="icon-grid">
          <li v-for="row in group.rows" :key="`${group.key}-${row.id}`">
            <component
              :is="selectable ? 'article' : RouterLink"
              class="project-tile"
              :class="{
                'project-tile-selectable': selectable,
                'project-tile-selected': selectable && selectedSet.has(row.id),
              }"
              v-bind="tileBindings(row)"
              @click="onTileClick($event, row.id)"
            >
              <label v-if="selectable" class="project-tile-check">
                <input
                  type="checkbox"
                  :checked="selectedSet.has(row.id)"
                  :aria-label="`Select ${row.name}`"
                  @change="toggleSelected(row.id)"
                >
              </label>
              <ProjectIcon size="lg" :src="projectIconURL(row.id, row.icon, row.iconRev)" />
              <RouterLink
                v-if="selectable"
                class="project-tile-name"
                :title="row.path"
                :to="{ name: 'project', query: { path: row.id } }"
              >{{ row.name }}</RouterLink>
              <span v-else class="project-tile-name">{{ row.name }}</span>
              <span v-if="row.tags" class="tag-list project-tile-tags">
                <span
                  v-for="tag in tagList(row.tags)"
                  :key="tag"
                  class="tag"
                  :style="tagChipStyle(tagColors, tag)"
                >{{ tag }}</span>
              </span>
            </component>
          </li>
        </ul>
      </div>
    </template>
  </div>
</template>
