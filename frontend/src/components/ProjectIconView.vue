<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import { sortRows } from 'picocrank/vue/composables/tableFilters.js'
import ProjectIcon from './ProjectIcon.vue'
import ProjectTag from './ProjectTag.vue'
import ProjectTagList from './ProjectTagList.vue'
import { projectIconURL } from '../composables/api'
import { applyProjectsFilterQuery, buildProjectsFilterQuery, splitCellTokens } from '../composables/projectTable'
import { loadIconViewState, saveIconViewState } from '../composables/projectView'
import {
  deleteProjectView,
  emptyProjectViewState,
  listProjectViews,
  mergeProjectViewState,
  sanitizeIconsViewState,
  saveProjectView,
  setDefaultProjectView,
  viewErrorMessage,
} from '../composables/projectViews'

const TEXT_OPERATORS = [
  { value: 'contains', label: 'Contains' },
  { value: 'equals', label: 'Equals' },
  { value: 'startsWith', label: 'Starts with' },
  { value: 'empty', label: 'Is empty' },
]

const PATH_TEXT_OPERATORS = [
  { value: 'contains', label: 'Excludes' },
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
  tagOrder: {
    type: Array,
    default: () => [],
  },
})

const emit = defineEmits(['update:selectedKeys', 'visible-paths-changed', 'layout-summary-changed'])

const savedIconView = loadIconViewState()
const sortBy = ref(savedIconView?.sortBy || 'name')
const sortDir = ref(savedIconView?.sortDir || 'asc')
const groupBy = ref(savedIconView?.groupBy || '')
const focusedGroupKey = ref(null)
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

const displayGroupsAll = computed(() => {
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

const displayGroups = computed(() => {
  const groups = displayGroupsAll.value
  if (!focusedGroupKey.value) return groups
  return groups.filter((group) => group.key === focusedGroupKey.value)
})

const gridVisibleRows = computed(() => {
  const seen = new Set()
  const out = []
  for (const group of displayGroups.value) {
    for (const row of group.rows) {
      if (seen.has(row.id)) continue
      seen.add(row.id)
      out.push(row)
    }
  }
  return out
})

watch(unusedFilterHeaders, (headers) => {
  if (!headers.some((header) => header.key === filterColumn.value)) {
    filterColumn.value = headers[0]?.key || ''
  }
}, { immediate: true })

function emptyDraft(header) {
  if (header?.filterType === 'select') return { type: 'select', values: [] }
  if (header?.filterType === 'number') return { type: 'number', operator: 'eq', value: '' }
  if (header?.filterType === 'boolean') return { type: 'boolean', value: null }
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
  if (fresh.type === 'boolean') {
    const value = draft.value
    return { type: 'boolean', value: value === true || value === false ? value : null }
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

watch(gridVisibleRows, (rows) => {
  emit('visible-paths-changed', rows.map((row) => row.id))
}, { immediate: true })

watch(groupBy, () => {
  focusedGroupKey.value = null
})

watch(displayGroupsAll, (groups) => {
  if (!focusedGroupKey.value) return
  if (!groups.some((group) => group.key === focusedGroupKey.value)) {
    focusedGroupKey.value = null
  }
})

function headerFor(key) {
  return headerByKey.value.get(key)
}

function textOperatorsFor(key) {
  return key === 'path' ? PATH_TEXT_OPERATORS : TEXT_OPERATORS
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

function setBooleanValue(key, value) {
  if (value === 'true') {
    writeDraft(key, { type: 'boolean', value: true })
    return
  }
  if (value === 'false') {
    writeDraft(key, { type: 'boolean', value: false })
    return
  }
  writeDraft(key, { type: 'boolean', value: null })
}

function booleanDraftValue(key) {
  const value = draftFor(key).value
  if (value === true) return 'true'
  if (value === false) return 'false'
  return ''
}

function toggleSortDir() {
  sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
}

const savedViews = ref([])
const defaultViewId = ref(null)
const selectedViewId = ref('')
const viewSaveName = ref('')
const viewSaveAsDefault = ref(false)
const viewMessage = ref('')
const viewError = ref('')
const viewBusy = ref(false)
const controlsDialogRef = ref(null)

const selectedView = computed(() =>
  savedViews.value.find((view) => String(view.id) === selectedViewId.value) || null,
)

const controlsSummary = computed(() => {
  const shownCount = gridVisibleRows.value.length
  const parts = [`${shownCount} shown`]
  if (filterRows.value.length) {
    const n = filterRows.value.length
    parts.push(`${n} filter${n === 1 ? '' : 's'}`)
  }
  if (groupBy.value) {
    const label = headerFor(groupBy.value)?.label || groupBy.value
    parts.push(`grouped by ${label}`)
  }
  if (focusedGroupKey.value) {
    const focused = displayGroupsAll.value.find((group) => group.key === focusedGroupKey.value)
    if (focused?.label) parts.push(`only “${focused.label}”`)
  }
  return parts.join(' · ')
})

function toggleGroupFocus(groupKey) {
  focusedGroupKey.value = focusedGroupKey.value === groupKey ? null : groupKey
}

function groupFocusLabel(group) {
  return `Show only ${group.label}. Click again to show all groups.`
}

function openControlsDialog() {
  controlsDialogRef.value?.showModal()
}

function onControlsDialogBackdropClick(event) {
  if (event.target === controlsDialogRef.value) {
    controlsDialogRef.value.close('close')
  }
}

watch(controlsSummary, (summary) => {
  emit('layout-summary-changed', summary)
}, { immediate: true })

async function refreshSavedViews() {
  try {
    const result = await listProjectViews()
    savedViews.value = result.views
    defaultViewId.value = result.defaultViewId
    if (!savedViews.value.some((view) => String(view.id) === selectedViewId.value)) {
      selectedViewId.value = savedViews.value[0] ? String(savedViews.value[0].id) : ''
    }
    viewError.value = ''
  } catch (err) {
    viewError.value = viewErrorMessage(err)
  }
}

function applyIconsState(state) {
  const sanitized = sanitizeIconsViewState(state)
  if (!sanitized) return
  sortBy.value = sanitized.sortBy
  sortDir.value = sanitized.sortDir
  groupBy.value = sanitized.groupBy
  const filters = sanitized.filters || []
  filterSeq = filters.length
  filterRows.value = filters.map((entry, index) => ({ id: index + 1, key: entry.key }))
  drafts.value = Object.fromEntries(
    filters
      .filter((entry) => entry.key && entry.draft)
      .map((entry) => [entry.key, alignDraft(entry.key, entry.draft)]),
  )
}

function getIconsState() {
  return {
    sortBy: sortBy.value,
    sortDir: sortDir.value,
    groupBy: groupBy.value,
    filters: filterRows.value.map((row) => ({
      key: row.key,
      draft: drafts.value[row.key],
    })),
  }
}

async function loadSelectedView() {
  const view = selectedView.value
  if (!view?.state?.icons) {
    viewError.value = 'This view has no icon layout saved yet.'
    return
  }
  viewBusy.value = true
  viewMessage.value = ''
  viewError.value = ''
  try {
    applyIconsState(view.state.icons)
    viewMessage.value = `Loaded “${view.name}”.`
  } finally {
    viewBusy.value = false
  }
}

async function saveCurrentView() {
  const name = viewSaveName.value.trim()
  if (!name) {
    viewError.value = 'Enter a name for this view.'
    return
  }
  viewBusy.value = true
  viewMessage.value = ''
  viewError.value = ''
  try {
    const result = await saveProjectView({
      name,
      state: mergeProjectViewState(emptyProjectViewState(), {
        icons: getIconsState(),
      }),
      setAsDefault: viewSaveAsDefault.value,
    })
    await refreshSavedViews()
    selectedViewId.value = String(result.view.id)
    viewSaveName.value = ''
    viewSaveAsDefault.value = false
    viewMessage.value = `Saved “${result.view.name}”.`
  } catch (err) {
    viewError.value = viewErrorMessage(err)
  } finally {
    viewBusy.value = false
  }
}

async function deleteSelectedView() {
  const view = selectedView.value
  if (!view) return
  viewBusy.value = true
  viewMessage.value = ''
  viewError.value = ''
  try {
    await deleteProjectView(view.id)
    await refreshSavedViews()
    viewMessage.value = `Deleted “${view.name}”.`
  } catch (err) {
    viewError.value = viewErrorMessage(err)
  } finally {
    viewBusy.value = false
  }
}

async function markSelectedViewDefault() {
  const view = selectedView.value
  if (!view) return
  viewBusy.value = true
  viewMessage.value = ''
  viewError.value = ''
  try {
    await setDefaultProjectView(view.id)
    await refreshSavedViews()
    viewMessage.value = `Set “${view.name}” as default.`
  } catch (err) {
    viewError.value = viewErrorMessage(err)
  } finally {
    viewBusy.value = false
  }
}

async function clearDefaultView() {
  viewBusy.value = true
  viewMessage.value = ''
  viewError.value = ''
  try {
    await setDefaultProjectView(0)
    await refreshSavedViews()
    viewMessage.value = 'Cleared default view.'
  } catch (err) {
    viewError.value = viewErrorMessage(err)
  } finally {
    viewBusy.value = false
  }
}

function visibleProjectIds() {
  return gridVisibleRows.value.map((row) => row.id)
}

defineExpose({
  applyIconsState,
  getIconsState,
  refreshSavedViews,
  visibleProjectIds,
  openControlsDialog,
})

onMounted(() => {
  refreshSavedViews()
})

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

</script>

<template>
  <div class="icon-view">
    <p v-if="gridVisibleRows.length === 0" class="scan-status padding">
      No projects match the current filters.
    </p>
    <template v-else>
      <div v-for="group in displayGroups" :key="group.key" class="icon-group">
        <h3 v-if="group.label" class="icon-group-title">
          <button
            type="button"
            class="icon-group-title-button"
            :class="{ 'icon-group-title-button-active': focusedGroupKey === group.key }"
            :aria-pressed="focusedGroupKey === group.key"
            :aria-label="groupFocusLabel(group)"
            @click="toggleGroupFocus(group.key)"
          >
            <ProjectTag
              v-if="groupBy === 'tags' && group.value"
              :tag="group.value"
              :tag-colors="tagColors"
            />
            <span v-else>{{ group.label }}</span>
            <span class="subtle">{{ group.rows.length }}</span>
          </button>
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
              <ProjectTagList
                v-if="row.tags"
                class="project-tile-tags"
                :tags="row.tags"
                :tag-colors="tagColors"
                :tag-order="tagOrder"
              />
            </component>
          </li>
        </ul>
      </div>
    </template>
  </div>

  <dialog
    ref="controlsDialogRef"
    class="root-dialog icon-view-controls-dialog"
    aria-labelledby="icon-view-controls-title"
    @click="onControlsDialogBackdropClick"
  >
    <form method="dialog" class="root-dialog-form icon-view-controls">
      <h2 id="icon-view-controls-title">Icon view layout</h2>
      <p class="subtle">Saved views, sort, group, and filters for the big icon grid.</p>

      <div class="icon-view-control-row icon-view-views-row">
        <label class="icon-view-control">
          <span class="icon-view-control-label">Views</span>
          <select v-model="selectedViewId" :disabled="!savedViews.length">
            <option v-if="!savedViews.length" value="">No saved views</option>
            <option v-for="view in savedViews" :key="view.id" :value="String(view.id)">
              {{ view.name }}{{ view.id === defaultViewId ? ' (default)' : '' }}
            </option>
          </select>
        </label>
        <div class="icon-view-inline-actions">
          <button
            type="button"
            class="neutral"
            :disabled="viewBusy || !selectedViewId"
            @click="loadSelectedView"
          >Load</button>
          <button
            type="button"
            class="neutral"
            :disabled="viewBusy || !selectedViewId"
            @click="deleteSelectedView"
          >Delete</button>
          <button
            v-if="selectedView && selectedView.id !== defaultViewId"
            type="button"
            class="neutral"
            :disabled="viewBusy || !selectedViewId"
            @click="markSelectedViewDefault"
          >Set default</button>
          <button
            v-else-if="defaultViewId"
            type="button"
            class="neutral"
            :disabled="viewBusy"
            @click="clearDefaultView"
          >Clear default</button>
        </div>
      </div>

      <div class="icon-view-control-row icon-view-views-save">
        <label class="icon-view-control">
          <span class="icon-view-control-label">Save as</span>
          <input
            v-model="viewSaveName"
            type="text"
            placeholder="View name"
            :disabled="viewBusy"
            @keydown.enter.prevent="saveCurrentView"
          >
        </label>
        <label class="icon-view-save-default">
          <input v-model="viewSaveAsDefault" type="checkbox" :disabled="viewBusy">
          <span>Default</span>
        </label>
        <button type="button" class="neutral" :disabled="viewBusy" @click="saveCurrentView">Save view</button>
      </div>

      <p v-if="viewMessage" class="subtle icon-view-views-message">{{ viewMessage }}</p>
      <p v-if="viewError" class="form-error icon-view-views-message">{{ viewError }}</p>

      <div class="icon-view-control-row">
        <label class="icon-view-control">
          <span class="icon-view-control-label">Sort</span>
          <select :value="activeSortBy" @change="sortBy = $event.target.value">
            <option v-for="header in sortableHeaders" :key="header.key" :value="header.key">
              {{ header.label }}
            </option>
          </select>
        </label>
        <button type="button" class="neutral" @click="toggleSortDir">
          {{ sortDir === 'asc' ? 'Ascending' : 'Descending' }}
        </button>
      </div>

      <div class="icon-view-control-row">
        <label class="icon-view-control">
          <span class="icon-view-control-label">Group</span>
          <select v-model="groupBy">
            <option value="">None</option>
            <option v-for="header in groupHeaders" :key="header.key" :value="header.key">
              {{ header.label }}
            </option>
          </select>
        </label>
        <span class="subtle icon-view-shown-count">{{ gridVisibleRows.length }} shown</span>
      </div>

      <div class="icon-view-control-row icon-view-filter-add">
        <label class="icon-view-control">
          <span class="icon-view-control-label">Filter</span>
          <select v-model="filterColumn" :disabled="unusedFilterHeaders.length === 0">
            <option v-for="header in unusedFilterHeaders" :key="header.key" :value="header.key">
              {{ header.label }}
            </option>
          </select>
        </label>
        <div class="icon-view-inline-actions">
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
      </div>

      <div
        v-for="row in filterRows"
        :key="row.id"
        class="icon-view-filter-row"
      >
        <div class="icon-view-filter-label">
          {{ headerFor(row.key)?.label || row.key }}
        </div>

        <div class="icon-view-filter-body">
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

          <template v-else-if="headerFor(row.key)?.filterType === 'boolean'">
            <select
              class="icon-view-filter-input"
              :value="booleanDraftValue(row.key)"
              @change="setBooleanValue(row.key, $event.target.value)"
            >
              <option value="">Any</option>
              <option value="true">Yes</option>
              <option value="false">No</option>
            </select>
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
                <option v-for="op in textOperatorsFor(row.key)" :key="op.value" :value="op.value">{{ op.label }}</option>
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

        <div class="icon-view-filter-actions">
          <button type="button" class="neutral" @click="removeFilter(row.id)">Remove</button>
        </div>
      </div>

      <fieldset class="icon-view-controls-close">
        <button type="submit" value="close" class="neutral">Close</button>
      </fieldset>
    </form>
  </dialog>
</template>
