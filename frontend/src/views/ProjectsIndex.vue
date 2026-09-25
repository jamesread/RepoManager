<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import Table from 'picocrank/vue/components/Table.vue'
import ProjectIcon from '../components/ProjectIcon.vue'
import ProjectIconView from '../components/ProjectIconView.vue'
import {
  buildDefaultColumnVisibility,
  fetchProjects,
  filterTypeForKind,
  formatCellValue,
  projectFieldValue,
  projectIconURL,
  refreshProject,
  refreshProjects,
  resolveFields,
  setProjectTags,
  setRoot,
  tagChipStyle,
  isHelperTag,
} from '../composables/api'
import {
  PROJECTS_TABLE_ID,
  collectDiscreteFilterOptions,
  collectExactFilterOptions,
  createLocalLayoutPresets,
  createProjectsFetchRows,
} from '../composables/projectTable'

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
const layoutPresets = createLocalLayoutPresets()
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
const tagMode = ref('add')
const tagDraft = ref('')
const tagDialogError = ref('')
const tagSaving = ref(false)
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
      col.filterOptions = collectExactFilterOptions(projects.value, f.id, projectFieldValue)
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

const removableTags = computed(() => {
  const selected = new Set(selectedKeys.value)
  const seen = new Set()
  for (const project of projects.value) {
    if (!selected.has(project.path)) continue
    for (const tag of project.tags || []) {
      if (tag && !isHelperTag(tag)) seen.add(tag)
    }
  }
  return [...seen].sort((a, b) => a.localeCompare(b))
})

const tagDialogTitle = computed(() => (tagMode.value === 'remove' ? 'Remove tag' : 'Add tag'))

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

async function onRefreshSelected() {
  const paths = [...selectedKeys.value]
  if (!paths.length || refreshingSelection.value) return
  refreshingSelection.value = true
  refreshing.value = {
    ...refreshing.value,
    ...Object.fromEntries(paths.map((path) => [path, true])),
  }
  try {
    await refreshProjects(paths)
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    refreshingSelection.value = false
    const next = { ...refreshing.value }
    for (const path of paths) delete next[path]
    refreshing.value = next
  }
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

async function openTagDialog(mode) {
  if (!selectedKeys.value.length) return
  tagMode.value = mode
  tagDialogError.value = ''
  tagDraft.value = mode === 'remove' ? (removableTags.value[0] || '') : ''
  tagDialogRef.value?.showModal()
  if (mode === 'remove') {
    await nextTick()
    tagDialogRef.value?.querySelector('#tag-name')?.focus()
  }
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

async function onTagDialogSubmit(event) {
  const submitter = event.submitter
  const action = submitter?.value || 'cancel'
  if (action !== 'save') return
  event.preventDefault()
  const tag = tagDraft.value.trim()
  if (!tag) {
    tagDialogError.value = 'Enter a tag'
    return
  }
  if (isHelperTag(tag)) {
    tagDialogError.value = 'Tag is set by a helper'
    return
  }
  if (!selectedKeys.value.length) {
    tagDialogError.value = 'Select at least one project'
    return
  }
  tagSaving.value = true
  tagDialogError.value = ''
  try {
    const data = await setProjectTags({
      paths: [...selectedKeys.value],
      tag,
      op: tagMode.value === 'remove' ? 'remove' : 'add',
    })
    applyProjectsPayload(data)
    error.value = ''
    rowsFingerprint = ''
    refreshTableIfRowsChanged()
    tagDialogRef.value?.close('save')
  } catch (e) {
    tagDialogError.value = e.message || String(e)
  } finally {
    tagSaving.value = false
  }
}

function toggleMultiSelect() {
  multiSelect.value = !multiSelect.value
  if (!multiSelect.value) selectedKeys.value = []
}

watch(viewMode, (mode) => {
  if (mode === 'icons' && !multiSelect.value) {
    selectedKeys.value = []
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
  if (tagDialogRef.value?.open || rootDialogRef.value?.open) return
  if (!selectedKeys.value.length) return
  event.preventDefault()
  openTagDialog('add')
}

function onProjectsUpdated(event) {
  if (!event.detail) return
  applyProjectsPayload(event.detail)
  error.value = ''
}

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
  <Section title="Projects" :subtitle="subtitle" :padding="false">
    <template #toolbar>
      <button
        v-if="viewMode === 'icons'"
        type="button"
        class="neutral multi-select-toggle"
        :aria-pressed="multiSelect"
        @click="toggleMultiSelect"
      >Multi select</button>
      <span v-if="selectedKeys.length" class="subtle">{{ selectedKeys.length }} selected</span>
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
        title="Add tag (t)"
        :disabled="tagSaving"
        @click="openTagDialog('add')"
      >Add tag…</button>
      <button
        v-if="selectedKeys.length"
        type="button"
        class="neutral"
        :disabled="tagSaving || removableTags.length === 0"
        @click="openTagDialog('remove')"
      >Remove tag…</button>
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
    <ProjectIconView
      v-else-if="viewMode === 'icons'"
      :rows="rows"
      :headers="headers"
      :selected-keys="selectedKeys"
      :selectable="multiSelect"
      :tag-colors="tagColors"
      @update:selected-keys="selectedKeys = $event"
    />
    <Table
      v-else
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
        <span v-if="value" class="tag-list">
          <span
            v-for="tag in String(value).split(', ').filter(Boolean)"
            :key="tag"
            class="tag"
            :style="tagChipStyle(tagColors, tag)"
          >{{ tag }}</span>
        </span>
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
    <form method="dialog" class="root-dialog-form" @submit="onTagDialogSubmit">
      <h2 id="tag-dialog-title">{{ tagDialogTitle }}</h2>
      <p class="subtle">{{ selectedKeys.length }} selected project{{ selectedKeys.length === 1 ? '' : 's' }}</p>
      <template v-if="tagMode === 'add'">
        <p class="subtle">Click a tag to turn it on or off for the selection. A dashed tag is only on some of the selected projects.</p>
        <div v-if="tagSuggestions.length" class="tag-toggle-list" role="group" aria-label="Available tags">
          <button
            v-for="tag in tagSuggestions"
            :key="tag"
            type="button"
            class="tag tag-toggle"
            :style="tagChipStyle(tagColors, tag)"
            :class="{
              'tag-toggle-on': tagState(tag) === 'on',
              'tag-toggle-mixed': tagState(tag) === 'mixed',
            }"
            :aria-pressed="tagPressed(tag)"
            :disabled="tagSaving"
            @click="toggleBulkTag(tag)"
          >{{ tag }}</button>
        </div>
        <p v-else class="subtle">No tags yet.</p>
      </template>
      <template v-else>
        <label for="tag-name">Tag</label>
        <select
          id="tag-name"
          v-model="tagDraft"
          :disabled="tagSaving"
        >
          <option v-for="tag in removableTags" :key="tag" :value="tag">{{ tag }}</option>
        </select>
      </template>
      <p v-if="tagDialogError" class="form-error">{{ tagDialogError }}</p>
      <fieldset>
        <button type="submit" value="cancel" class="neutral" :disabled="tagSaving">
          {{ tagMode === 'add' ? 'Close' : 'Cancel' }}
        </button>
        <button
          v-if="tagMode === 'remove'"
          type="submit"
          value="save"
          class="bad"
          :disabled="tagSaving || removableTags.length === 0"
        >
          {{ tagSaving ? 'Saving…' : 'Remove tag' }}
        </button>
      </fieldset>
    </form>
  </dialog>
</template>
