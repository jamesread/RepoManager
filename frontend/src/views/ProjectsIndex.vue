<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { CopyIcon, GithubIcon, RefreshIcon } from '@hugeicons/core-free-icons'
import Section from 'picocrank/vue/components/Section.vue'
import Table from 'picocrank/vue/components/Table.vue'
import ProjectIcon from '../components/ProjectIcon.vue'
import ProjectIconView from '../components/ProjectIconView.vue'
import ProjectTag from '../components/ProjectTag.vue'
import ProjectTagList from '../components/ProjectTagList.vue'
import {
  buildDefaultColumnVisibility,
  fetchProjects,
  filterTypeForKind,
  formatCellValue,
  githubProjectURL,
  projectFieldValue,
  projectIconURL,
  refreshProject,
  refreshProjects,
  resolveFields,
  setProjectAnnotation,
  setProjectTags,
  setRoot,
  isHelperTag,
} from '../composables/api'
import {
  PROJECTS_TABLE_ID,
  collectDiscreteFilterOptions,
  collectAnnotationFilterOptions,
  buildProjectsSpreadsheetTSV,
  createProjectsFetchRows,
  visibleFilteredProjectIds,
} from '../composables/projectTable'
import { createServerProjectViews } from '../composables/projectViews'

const route = useRoute()
const viewMode = computed(() => (route.query.view === 'icons' ? 'icons' : 'table'))
const multiSelect = ref(false)

const root = ref('')
const recentRoots = ref([])
const scanning = ref(false)
const found = ref(0)
const projects = ref([])
const fields = ref(resolveFields([]))
const error = ref('')
const columnVisibility = ref(buildDefaultColumnVisibility([]))
const tableFilters = ref({})
const iconViewRef = ref(null)
const iconsLayoutSummary = ref('')
const layoutPresets = createServerProjectViews({
  getIconsState: () => iconViewRef.value?.getIconsState?.(),
  onApplyIcons: (icons) => iconViewRef.value?.applyIconsState?.(icons),
  getHeaders: () => headers.value,
  getRows: () => rows.value,
})
const refreshing = ref({})
const refreshingSelection = ref(false)
const rootDialogRef = ref(null)
const rootDraft = ref('')
const rootDialogError = ref('')
const rootSaving = ref(false)
const selectedKeys = ref([])
const knownTags = ref([])
const tagColors = ref({})
const tagDialogRef = ref(null)
const tagDialogError = ref('')
const tagSaving = ref(false)
const annotationDialogRef = ref(null)
const annotationDrafts = ref({})
const annotationMixed = ref({})
const annotationSnapshot = ref({})
const newAnnotationKey = ref('')
const newAnnotationValue = ref('')
const annotationDialogError = ref('')
const annotationSaving = ref(false)
const copyTableStatus = ref('')
let timer = null
let visibilityReady = false
let rowsFingerprint = ''

function currentRowsFingerprint() {
  return rows.value
    .map((row) => `${row.id}\t${row.icon || ''}\t${row.iconRev || ''}\t${row.tags || ''}\t${row.modules || ''}\t${row['dir.mtime'] || ''}\t${row['git.last_commit'] || ''}`)
    .join('\n')
}

function bumpTableQuery() {
  // fetchRows mode only refetches on filter/page/sort changes; poke filters to reload.
  tableFilters.value = JSON.parse(JSON.stringify(tableFilters.value))
}

function refreshTableIfRowsChanged() {
  const next = currentRowsFingerprint()
  if (next === rowsFingerprint) {
    return
  }
  rowsFingerprint = next
  bumpTableQuery()
}

const headers = computed(() => {
  const catalog = fields.value || []
  const metaFields = catalog.filter((f) => f.id !== 'name' && f.id !== 'tags' && f.id !== 'icon')
  const tagsField = catalog.find((f) => f.id === 'tags')

  const cols = [
    {
      key: 'name',
      label: 'Name',
      sortable: true,
      filterable: true,
      filterType: 'text',
      hideable: false,
    },
    {
      key: 'path',
      label: 'Path',
      sortable: true,
      filterable: true,
      filterType: 'text',
    },
  ]

  for (const f of metaFields) {
    const annotation = f.id.startsWith('annotation.')
    const col = {
      key: f.id,
      label: f.label,
      sortable: !!f.sortable,
      filterable: f.filterable !== false,
      filterType: annotation ? 'select' : filterTypeForKind(f.kind),
    }
    if (f.kind === 'multi') {
      col.multiValue = true
      col.filterOptions = collectDiscreteFilterOptions(projects.value, f.id, projectFieldValue)
    } else if (annotation) {
      col.filterOptions = collectAnnotationFilterOptions(projects.value, f.id, projectFieldValue)
    }
    cols.push(col)
  }

  cols.push({
    key: 'tags',
    label: tagsField?.label || 'Tags',
    sortable: false,
    filterable: true,
    filterType: 'select',
    multiValue: true,
    filterOptions: collectDiscreteFilterOptions(projects.value, 'tags', projectFieldValue),
  })

  cols.push({
    key: 'actions',
    label: 'Actions',
    sortable: false,
    filterable: false,
    hideable: false,
    orderable: false,
    class: 'actions',
    width: '4rem',
  })

  return cols
})

const defaultColumnVisibility = computed(() => buildDefaultColumnVisibility(headers.value))

const rows = computed(() => {
  const catalog = fields.value || []
  const kindByID = Object.fromEntries(catalog.map((f) => [f.id, f.kind]))
  kindByID.path = 'string'
  kindByID.name = 'string'

  return projects.value.map((p) => {
    const row = { id: p.path }
    for (const header of headers.value) {
      if (header.key === 'actions') {
        row.actions = ''
        continue
      }
      const raw = projectFieldValue(p, header.key)
      if (header.key === 'tags' || kindByID[header.key] === 'multi') {
        row[header.key] = Array.isArray(raw)
          ? raw.join(', ')
          : formatCellValue(kindByID[header.key], raw)
        continue
      }
      if (kindByID[header.key] === 'time') {
        row[header.key] = formatCellValue('time', raw)
        continue
      }
      if (kindByID[header.key] === 'int' && raw != null && raw !== '') {
        row[header.key] = Number(raw)
        continue
      }
      if (kindByID[header.key] === 'bool') {
        row[header.key] = typeof raw === 'boolean' ? raw : ''
        continue
      }
      row[header.key] = raw == null ? '' : raw
    }
    row.icon = projectFieldValue(p, 'icon') || ''
    row.iconRev = p.meta?.['icon.rev'] || ''
    return row
  })
})

const fetchRows = createProjectsFetchRows(
  () => rows.value,
  () => headers.value,
)

const subtitle = computed(() => {
  if (!root.value) return 'Loading…'
  const count = `${found.value || projects.value.length} projects`
  return scanning.value ? `scanning · ${count}` : count
})

const previousRoots = computed(() =>
  (recentRoots.value || []).filter((path) => path && path !== root.value),
)

const tagSuggestions = computed(() => {
  const seen = new Set(knownTags.value || [])
  for (const project of projects.value) {
    for (const tag of project.tags || []) {
      if (tag) seen.add(tag)
    }
  }
  return [...seen].filter((tag) => !isHelperTag(tag)).sort((a, b) => a.localeCompare(b))
})

const annotationKeys = computed(() => {
  const keys = []
  const seen = new Set()
  for (const field of fields.value || []) {
    if (!field.id?.startsWith('annotation.')) continue
    const key = field.id.slice('annotation.'.length)
    if (key && !seen.has(key)) {
      seen.add(key)
      keys.push(key)
    }
  }
  for (const project of projects.value) {
    for (const key of Object.keys(project.annotations || {})) {
      if (key && !seen.has(key)) {
        seen.add(key)
        keys.push(key)
      }
    }
  }
  return keys.sort((a, b) => a.localeCompare(b))
})

function applyProjectsPayload(data) {
  root.value = data.root || ''
  recentRoots.value = Array.isArray(data.recent_roots) ? data.recent_roots : []
  knownTags.value = Array.isArray(data.known_tags) ? data.known_tags : []
  tagColors.value = data.tag_colors && typeof data.tag_colors === 'object' ? data.tag_colors : {}
  scanning.value = !!data.scanning
  found.value = data.found || 0
  fields.value = resolveFields(data.fields)
  projects.value = data.projects || []
  const alive = new Set(projects.value.map((project) => project.path))
  selectedKeys.value = selectedKeys.value.filter((key) => alive.has(key))
  syncColumnVisibility(headers.value)
  refreshTableIfRowsChanged()
}

function syncColumnVisibility(nextHeaders) {
  const defaults = buildDefaultColumnVisibility(nextHeaders)
  if (!visibilityReady) {
    columnVisibility.value = { ...defaults }
    visibilityReady = true
    return
  }
  const merged = { ...defaults }
  for (const [key, value] of Object.entries(columnVisibility.value)) {
    if (Object.prototype.hasOwnProperty.call(merged, key)) {
      merged[key] = value
    }
  }
  columnVisibility.value = merged
}

function isRefreshing(path) {
  return !!refreshing.value[path]
}

async function onRefreshProject(path) {
  if (!path || isRefreshing(path)) {
    return
  }
  refreshing.value = { ...refreshing.value, [path]: true }
  try {
    await refreshProject(path)
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    const next = { ...refreshing.value }
    delete next[path]
    refreshing.value = next
  }
}

async function copySelectedTableToClipboard() {
  copyTableStatus.value = ''
  if (!selectedKeys.value.length) {
    return
  }
  const text = buildProjectsSpreadsheetTSV(
    rows.value,
    headers.value,
    columnVisibility.value,
    selectedKeys.value,
  )
  if (!text.trim()) {
    copyTableStatus.value = 'Nothing to copy'
    return
  }
  try {
    await navigator.clipboard.writeText(text)
    copyTableStatus.value = 'Copied to clipboard'
  } catch (e) {
    copyTableStatus.value = e.message || String(e)
  }
}

async function onRefreshPaths(paths) {
  const targets = [...paths].filter(Boolean)
  if (!targets.length) return
  if (targets.length === 1) {
    await onRefreshProject(targets[0])
    return
  }
  if (refreshingSelection.value) return
  refreshingSelection.value = true
  refreshing.value = {
    ...refreshing.value,
    ...Object.fromEntries(targets.map((path) => [path, true])),
  }
  try {
    await refreshProjects(targets)
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    refreshingSelection.value = false
    const next = { ...refreshing.value }
    for (const path of targets) delete next[path]
    refreshing.value = next
  }
}

async function onRefreshSelected() {
  await onRefreshPaths(selectedKeys.value)
}

async function copyProjectPaths(paths) {
  const text = [...paths].filter(Boolean).join('\n')
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
  } catch (e) {
    error.value = e.message || String(e)
  }
}

function projectByPath(path) {
  return projects.value.find((project) => project.path === path) || null
}

function githubURLsForPaths(paths) {
  return [...paths]
    .map((path) => githubProjectURL(projectByPath(path)))
    .filter(Boolean)
}

function openGitHubForPaths(paths) {
  for (const url of githubURLsForPaths(paths)) {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

function projectCardContextMenu(ctx) {
  const paths = ctx.keys?.length ? ctx.keys : [ctx.key]
  const busy = refreshingSelection.value || paths.some((path) => isRefreshing(path))
  const githubCount = githubURLsForPaths(paths).length
  const items = [
    {
      id: 'refresh',
      label: busy ? 'Refreshing…' : 'Refresh',
      icon: RefreshIcon,
      disabled: busy,
      action: ({ keys }) => {
        onRefreshPaths(keys)
      },
    },
    {
      id: 'copy-path',
      label: paths.length > 1 ? 'Copy paths' : 'Copy path',
      icon: CopyIcon,
      showCount: false,
      action: ({ keys }) => {
        copyProjectPaths(keys)
      },
    },
  ]
  if (githubCount > 0) {
    items.push({
      id: 'open-github',
      label: githubCount > 1 ? `Open GitHub (${githubCount})` : 'Open GitHub',
      icon: GithubIcon,
      showCount: false,
      action: ({ keys }) => {
        openGitHubForPaths(keys)
      },
    })
  }
  return items
}

async function load() {
  try {
    const data = await fetchProjects()
    applyProjectsPayload(data)
    error.value = ''
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function openRootDialog() {
  rootDraft.value = root.value || ''
  rootDialogError.value = ''
  rootDialogRef.value?.showModal()
  await nextTick()
  const input = rootDialogRef.value?.querySelector('input[name="root"]')
  input?.focus()
  input?.select()
}

function onRootDialogBackdropClick(event) {
  if (event.target === rootDialogRef.value) {
    rootDialogRef.value.close('cancel')
  }
}

async function chooseRoot(nextRoot) {
  const trimmed = nextRoot.trim()
  if (!trimmed) {
    rootDialogError.value = 'Enter a directory path'
    return
  }
  rootSaving.value = true
  rootDialogError.value = ''
  try {
    const data = await setRoot(trimmed)
    applyProjectsPayload(data)
    error.value = ''
    rowsFingerprint = ''
    refreshTableIfRowsChanged()
    rootDialogRef.value?.close('save')
  } catch (e) {
    rootDialogError.value = e.message || String(e)
  } finally {
    rootSaving.value = false
  }
}

async function onRootDialogSubmit(event) {
  const submitter = event.submitter
  const action = submitter?.value || 'cancel'
  if (action !== 'save') {
    return
  }
  event.preventDefault()
  await chooseRoot(rootDraft.value)
}

function tagState(tag) {
  const selected = new Set(selectedKeys.value)
  let total = 0
  let hits = 0
  for (const project of projects.value) {
    if (!selected.has(project.path)) continue
    total++
    if ((project.tags || []).includes(tag)) hits++
  }
  if (total === 0 || hits === 0) return 'off'
  if (hits === total) return 'on'
  return 'mixed'
}

function tagPressed(tag) {
  const state = tagState(tag)
  if (state === 'mixed') return 'mixed'
  return state === 'on'
}

function pruneSelectedKeysToVisibleView() {
  const visible = viewMode.value === 'icons'
    ? new Set(iconViewRef.value?.visibleProjectIds?.() || [])
    : visibleFilteredProjectIds(rows.value, headers.value, tableFilters.value)
  if (visible.size === 0) {
    selectedKeys.value = []
    return 0
  }
  const before = selectedKeys.value.length
  selectedKeys.value = selectedKeys.value.filter((key) => visible.has(key))
  return before - selectedKeys.value.length
}

function onIconVisiblePathsChanged(paths) {
  if (viewMode.value !== 'icons') return
  const visible = new Set(paths)
  selectedKeys.value = selectedKeys.value.filter((key) => visible.has(key))
}

function annotationBulkValue(key) {
  const selected = new Set(selectedKeys.value)
  let value = null
  let mixed = false
  for (const project of projects.value) {
    if (!selected.has(project.path)) continue
    const current = project.annotations?.[key] || ''
    if (value === null) {
      value = current
      continue
    }
    if (value !== current) {
      mixed = true
      break
    }
  }
  return { value: value ?? '', mixed }
}

function initAnnotationDrafts() {
  const drafts = {}
  const mixed = {}
  const snapshot = {}
  for (const key of annotationKeys.value) {
    const state = annotationBulkValue(key)
    drafts[key] = state.mixed ? '' : state.value
    mixed[key] = state.mixed
    snapshot[key] = state.mixed ? '__mixed__' : state.value
  }
  annotationDrafts.value = drafts
  annotationMixed.value = mixed
  annotationSnapshot.value = snapshot
}

async function openTagDialog() {
  if (!selectedKeys.value.length) return
  tagDialogError.value = ''
  tagDialogRef.value?.showModal()
}

async function openAnnotationDialog() {
  if (!selectedKeys.value.length) return
  const dropped = pruneSelectedKeysToVisibleView()
  if (!selectedKeys.value.length) {
    annotationDialogError.value = dropped > 0
      ? 'No selected projects match the current filters.'
      : ''
    return
  }
  annotationDialogError.value = ''
  newAnnotationKey.value = ''
  newAnnotationValue.value = ''
  initAnnotationDrafts()
  annotationDialogRef.value?.showModal()
}

async function toggleBulkTag(tag) {
  if (!tag || !selectedKeys.value.length || tagSaving.value || isHelperTag(tag)) return
  const op = tagState(tag) === 'on' ? 'remove' : 'add'
  tagSaving.value = true
  tagDialogError.value = ''
  try {
    const data = await setProjectTags({
      paths: [...selectedKeys.value],
      tag,
      op,
    })
    applyProjectsPayload(data)
    error.value = ''
    rowsFingerprint = ''
    refreshTableIfRowsChanged()
  } catch (e) {
    tagDialogError.value = e.message || String(e)
  } finally {
    tagSaving.value = false
  }
}

function onTagDialogBackdropClick(event) {
  if (event.target === tagDialogRef.value) {
    tagDialogRef.value.close('cancel')
  }
}

function onAnnotationDialogBackdropClick(event) {
  if (event.target === annotationDialogRef.value) {
    annotationDialogRef.value.close('cancel')
  }
}

function onAnnotationDialogClosed() {
  selectedKeys.value = []
}

async function saveBulkAnnotation(key) {
  if (!key || annotationSaving.value || !selectedKeys.value.length) return
  const value = String(annotationDrafts.value[key] ?? '').trim()
  const snapshot = annotationSnapshot.value[key]
  if (snapshot === '__mixed__' && value === '') return
  if (snapshot !== '__mixed__' && value === snapshot) return
  annotationSaving.value = true
  annotationDialogError.value = ''
  try {
    const paths = [...selectedKeys.value]
    await Promise.all(paths.map((path) => setProjectAnnotation({ path, key, value })))
    await load()
    annotationMixed.value = { ...annotationMixed.value, [key]: false }
    annotationSnapshot.value = { ...annotationSnapshot.value, [key]: value }
  } catch (e) {
    annotationDialogError.value = e.message || String(e)
  } finally {
    annotationSaving.value = false
  }
}

async function addBulkAnnotation() {
  const key = newAnnotationKey.value.trim()
  if (!key || annotationSaving.value || !selectedKeys.value.length) return
  const value = newAnnotationValue.value.trim()
  annotationSaving.value = true
  annotationDialogError.value = ''
  try {
    const paths = [...selectedKeys.value]
    await Promise.all(paths.map((path) => setProjectAnnotation({ path, key, value })))
    newAnnotationKey.value = ''
    newAnnotationValue.value = ''
    await load()
    initAnnotationDrafts()
  } catch (e) {
    annotationDialogError.value = e.message || String(e)
  } finally {
    annotationSaving.value = false
  }
}

function openIconLayoutDialog() {
  iconViewRef.value?.openControlsDialog?.()
}

function toggleMultiSelect() {
  multiSelect.value = !multiSelect.value
  if (!multiSelect.value) selectedKeys.value = []
}

watch(viewMode, (mode) => {
  if (mode === 'icons' && !multiSelect.value) {
    selectedKeys.value = []
  }
  if (mode !== 'table') {
    copyTableStatus.value = ''
  }
})

watch(selectedKeys, (keys) => {
  if (!keys.length) {
    copyTableStatus.value = ''
  }
})

function onPageKeydown(event) {
  if (event.key !== 't' || event.metaKey || event.ctrlKey || event.altKey) return
  if (viewMode.value === 'icons' && !multiSelect.value) return
  const el = event.target
  if (el instanceof HTMLElement) {
    const tagName = el.tagName
    if (tagName === 'INPUT' || tagName === 'TEXTAREA' || tagName === 'SELECT' || el.isContentEditable) return
  }
  if (tagDialogRef.value?.open || annotationDialogRef.value?.open || rootDialogRef.value?.open) return
  if (!selectedKeys.value.length) return
  event.preventDefault()
  openTagDialog()
}

function onProjectsUpdated(event) {
  if (!event.detail) return
  applyProjectsPayload(event.detail)
  error.value = ''
}

watch(tableFilters, () => {
  if (viewMode.value === 'table') {
    pruneSelectedKeysToVisibleView()
  }
}, { deep: true })

onMounted(() => {
  syncColumnVisibility(headers.value)
  load()
  timer = setInterval(load, 1500)
  window.addEventListener('keydown', onPageKeydown)
  window.addEventListener('plist-projects', onProjectsUpdated)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  window.removeEventListener('keydown', onPageKeydown)
  window.removeEventListener('plist-projects', onProjectsUpdated)
})
</script>

<template>
  <Section
    title="Projects"
    :subtitle="subtitle"
    :padding="false"
    :classes="viewMode === 'icons' ? 'projects-icons-view' : ''"
  >
    <template #toolbar>
      <button
        v-if="viewMode === 'icons'"
        type="button"
        class="neutral"
        @click="openIconLayoutDialog"
      >Layout &amp; filters…</button>
      <span
        v-if="viewMode === 'icons' && iconsLayoutSummary"
        class="subtle projects-icons-layout-summary"
      >{{ iconsLayoutSummary }}</span>
      <button
        v-if="viewMode === 'icons'"
        type="button"
        class="neutral multi-select-toggle"
        :aria-pressed="multiSelect"
        @click="toggleMultiSelect"
      >Multi select</button>
      <span v-if="selectedKeys.length" class="subtle">{{ selectedKeys.length }} selected</span>
      <button
        v-if="viewMode === 'table' && selectedKeys.length"
        type="button"
        class="neutral"
        title="Copy visible columns and selected rows as tab-separated values for a spreadsheet"
        @click="copySelectedTableToClipboard"
      >Copy table</button>
      <span
        v-if="viewMode === 'table' && copyTableStatus"
        class="subtle"
      >{{ copyTableStatus }}</span>
      <button
        v-if="selectedKeys.length"
        type="button"
        class="neutral"
        :disabled="refreshingSelection || tagSaving"
        @click="onRefreshSelected"
      >{{ refreshingSelection ? 'Refreshing…' : 'Refresh' }}</button>
      <button
        v-if="selectedKeys.length"
        type="button"
        class="good"
        title="Tags (t)"
        :disabled="tagSaving"
        @click="openTagDialog"
      >Tags…</button>
      <button
        v-if="viewMode === 'icons' && selectedKeys.length"
        type="button"
        class="neutral"
        :disabled="annotationSaving"
        @click="openAnnotationDialog"
      >Annotations…</button>
      <button
        v-if="selectedKeys.length"
        type="button"
        class="neutral"
        :disabled="tagSaving"
        @click="selectedKeys = []"
      >Clear</button>
      <span v-if="root" class="root-toolbar-path" :title="root">{{ root }}</span>
      <button
        type="button"
        class="neutral"
        :disabled="rootSaving"
        @click="openRootDialog"
      >Change root…</button>
    </template>

    <p v-if="error" class="form-error padding">{{ error }}</p>
    <p v-else-if="projects.length === 0" class="scan-status padding">
      {{ scanning ? 'Scanning for Git projects…' : 'No Git projects found under this root.' }}
    </p>
    <div v-else class="projects-layout-views">
      <ProjectIconView
        v-show="viewMode === 'icons'"
        ref="iconViewRef"
        :rows="rows"
        :headers="headers"
        :selected-keys="selectedKeys"
        :selectable="multiSelect"
        :tag-colors="tagColors"
        :tag-order="knownTags"
        :row-context-menu="projectCardContextMenu"
        @update:selected-keys="selectedKeys = $event"
        @visible-paths-changed="onIconVisiblePathsChanged"
        @layout-summary-changed="iconsLayoutSummary = $event"
      />
      <Table
        v-show="viewMode === 'table'"
      :table-id="PROJECTS_TABLE_ID"
      row-key="id"
      :data="rows"
      :headers="headers"
      :column-options="true"
      v-model:column-visibility="columnVisibility"
      v-model:filters="tableFilters"
      :default-column-visibility="defaultColumnVisibility"
      :fetch-rows="fetchRows"
      :layout-presets="layoutPresets"
      :remote-debounce-ms="50"
      :horizontal-scroll="true"
      :responsive-columns="false"
      :sticky-cols="2"
      selectable
      v-model:selected-keys="selectedKeys"
    >
      <template #cell-name="{ row, value }">
        <span class="project-name">
          <ProjectIcon :src="projectIconURL(row.id, row.icon, row.iconRev)" />
          <RouterLink
            class="project-name-link"
            :to="{ name: 'project', query: { path: row.id } }"
          >{{ value }}</RouterLink>
        </span>
      </template>
      <template #cell-path="{ value }">
        <span class="project-path">{{ value }}</span>
      </template>
      <template #cell-modules="{ value }">
        <span v-if="value" class="tag-list">
          <span
            v-for="mod in String(value).split(', ').filter(Boolean)"
            :key="mod"
            class="tag"
          >{{ mod }}</span>
        </span>
        <span v-else>—</span>
      </template>
      <template #cell-tags="{ value }">
        <ProjectTagList v-if="value" :tags="value" :tag-colors="tagColors" :tag-order="knownTags" />
        <span v-else>—</span>
      </template>
      <template #cell-actions="{ row }">
        <nav class="menu" @click.stop>
          <ul>
            <li>
              <span class="actions-menu-trigger">
                <button
                  type="button"
                  aria-label="Actions"
                  aria-haspopup="menu"
                  :disabled="isRefreshing(row.id)"
                >⋯</button>
              </span>
              <div role="menu">
                <button
                  type="button"
                  role="menuitem"
                  class="neutral"
                  :disabled="isRefreshing(row.id)"
                  @click="onRefreshProject(row.id)"
                >{{ isRefreshing(row.id) ? 'Refreshing…' : 'Refresh' }}</button>
              </div>
            </li>
          </ul>
        </nav>
      </template>
    </Table>
    </div>
  </Section>

  <dialog
    ref="rootDialogRef"
    class="root-dialog"
    aria-labelledby="root-dialog-title"
    @click="onRootDialogBackdropClick"
  >
    <form method="dialog" class="root-dialog-form" @submit="onRootDialogSubmit">
      <h2 id="root-dialog-title">Change listing root</h2>
      <p class="subtle">Scan for Git projects under this directory.</p>
      <label for="root-path">Directory</label>
      <input
        id="root-path"
        v-model="rootDraft"
        type="text"
        name="root"
        autocomplete="off"
        spellcheck="false"
        placeholder="/path/to/projects"
        :disabled="rootSaving"
      >
      <p v-if="rootDialogError" class="form-error">{{ rootDialogError }}</p>
      <div v-if="previousRoots.length" class="recent-roots">
        <p class="subtle">Previous roots</p>
        <ul>
          <li v-for="path in previousRoots" :key="path">
            <button
              type="button"
              class="neutral"
              :title="path"
              :disabled="rootSaving"
              @click="chooseRoot(path)"
            >{{ path }}</button>
          </li>
        </ul>
      </div>
      <fieldset>
        <button type="submit" value="cancel" class="neutral" :disabled="rootSaving">Cancel</button>
        <button type="submit" value="save" class="good" :disabled="rootSaving">
          {{ rootSaving ? 'Changing…' : 'Change root' }}
        </button>
      </fieldset>
    </form>
  </dialog>

  <dialog
    ref="tagDialogRef"
    class="root-dialog"
    aria-labelledby="tag-dialog-title"
    @click="onTagDialogBackdropClick"
  >
    <form method="dialog" class="root-dialog-form">
      <h2 id="tag-dialog-title">Tags</h2>
      <p class="subtle">{{ selectedKeys.length }} selected project{{ selectedKeys.length === 1 ? '' : 's' }}</p>
      <p class="subtle">Click a tag to turn it on or off for the selection. A dashed tag is only on some of the selected projects.</p>
      <div v-if="tagSuggestions.length" class="tag-toggle-list" role="group" aria-label="Available tags">
        <ProjectTag
          v-for="tag in tagSuggestions"
          :key="tag"
          :tag="tag"
          :tag-colors="tagColors"
          toggle
          :pressed="tagPressed(tag)"
          :mixed="tagState(tag) === 'mixed'"
          :disabled="tagSaving"
          @click="toggleBulkTag(tag)"
        />
      </div>
      <p v-else class="subtle">No tags yet.</p>
      <p v-if="tagDialogError" class="form-error">{{ tagDialogError }}</p>
      <fieldset>
        <button type="submit" value="cancel" class="neutral" :disabled="tagSaving">Close</button>
      </fieldset>
    </form>
  </dialog>

  <dialog
    ref="annotationDialogRef"
    class="root-dialog"
    aria-labelledby="annotation-dialog-title"
    @click="onAnnotationDialogBackdropClick"
    @close="onAnnotationDialogClosed"
  >
    <form class="root-dialog-form" @submit.prevent="addBulkAnnotation">
      <h2 id="annotation-dialog-title">Annotations</h2>
      <p class="subtle">{{ selectedKeys.length }} selected project{{ selectedKeys.length === 1 ? '' : 's' }}</p>
      <p class="subtle">Edit a field and tab out to apply it to every selected project.</p>
      <div v-if="annotationKeys.length" class="annotation-list">
        <label v-for="key in annotationKeys" :key="key" class="annotation-row">
          <span>{{ key }}</span>
          <input
            v-model="annotationDrafts[key]"
            type="text"
            :placeholder="annotationMixed[key] ? '(mixed)' : ''"
            :disabled="annotationSaving"
            @blur="saveBulkAnnotation(key)"
          >
        </label>
      </div>
      <p v-else class="subtle">No annotations yet.</p>
      <div class="detail-edit-add">
        <input
          v-model="newAnnotationKey"
          type="text"
          placeholder="New annotation key"
          aria-label="New annotation key"
          :disabled="annotationSaving"
        >
        <input
          v-model="newAnnotationValue"
          type="text"
          placeholder="Value"
          aria-label="New annotation value"
          :disabled="annotationSaving"
        >
        <button type="submit" class="neutral" :disabled="annotationSaving || !newAnnotationKey.trim()">
          {{ annotationSaving ? 'Saving…' : 'Add annotation' }}
        </button>
      </div>
      <p v-if="annotationDialogError" class="form-error">{{ annotationDialogError }}</p>
      <fieldset>
        <button type="button" class="neutral" :disabled="annotationSaving" @click="annotationDialogRef?.close('cancel')">
          Close
        </button>
      </fieldset>
    </form>
  </dialog>
</template>
