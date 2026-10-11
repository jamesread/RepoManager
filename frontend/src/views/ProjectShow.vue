<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import Section from 'picocrank/vue/components/Section.vue'
import DangerZone from 'picocrank/vue/components/DangerZone.vue'
import ProjectIcon from '../components/ProjectIcon.vue'
import ProjectTag from '../components/ProjectTag.vue'
import ProjectTagList from '../components/ProjectTagList.vue'
import {
  deleteProject,
  fetchProject,
  formatCellValue,
  isProjectNotFoundError,
  projectFieldValue,
  projectIconURL,
  refreshProject,
  resolveFields,
  setProjectAnnotation,
  setProjectTags,
  isHelperTag,
} from '../composables/api'
import { projectsListLocation } from '../composables/projectView'

const props = defineProps({
  path: {
    type: String,
    required: true,
  },
})

const route = useRoute()
const router = useRouter()
const root = ref('')
const fields = ref(resolveFields([]))
const project = ref(null)
const knownTags = ref([])
const tagColors = ref({})
const error = ref('')
const missing = ref(false)
const editError = ref('')
const loading = ref(true)
const refreshing = ref(false)
const deleting = ref(false)
const deleteDialogRef = ref(null)
const editDialogRef = ref(null)
const deleteError = ref('')
const saving = ref(false)
const tagDraft = ref('')
const newAnnotationKey = ref('')
const newAnnotationValue = ref('')
const annotationDrafts = ref({})
const activeEdit = ref('')
let timer = null
let loadSeq = 0

const title = computed(() => {
  if (missing.value) {
    const parts = String(props.path || '').split(/[/\\]/).filter(Boolean)
    return parts[parts.length - 1] || 'Project'
  }
  return project.value?.rel || 'Project'
})
const backTo = computed(() => projectsListLocation())
const hasLocalChanges = computed(() => (project.value?.tags || []).includes('changes'))

const iconURL = computed(() => projectIconURL(
  project.value?.path,
  project.value?.meta?.icon,
  project.value?.meta?.['icon.rev'],
))

const detailRows = computed(() => {
  if (!project.value) return []
  const catalog = fields.value || []
  const tagsField = catalog.find((f) => f.id === 'tags')
  const rows = [
    { id: 'path', label: 'Path', kind: 'string', value: project.value.path },
    {
      id: 'tags',
      label: tagsField?.label || 'Tags',
      kind: 'multi',
      value: project.value.tags || [],
    },
  ]
  for (const key of annotationKeys.value) {
    const value = project.value.annotations?.[key] || ''
    if (!value) continue
    rows.push({
      id: `annotation.${key}`,
      label: `@${key}`,
      kind: 'string',
      value,
    })
  }
  for (const f of catalog) {
    if (f.id === 'name' || f.id === 'icon' || f.id === 'tags' || f.id.startsWith('annotation.')) continue
    const raw = projectFieldValue(project.value, f.id)
    if (raw == null || raw === '' || (Array.isArray(raw) && raw.length === 0)) {
      continue
    }
    rows.push({
      id: f.id,
      label: f.label,
      kind: f.kind,
      value: raw,
    })
  }
  return rows
})

function packageURLHref(value) {
  const text = String(value || '').trim()
  if (!text) return ''
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(text)) return text
  return `https://${text}`
}

const githubURL = computed(() => {
  const meta = project.value?.meta || {}
  const owner = meta['github.owner']
  const repo = meta['github.repo']
  if (owner && repo) {
    return `https://github.com/${owner}/${repo}`
  }
  const full = meta['github.full_name']
  if (full) {
    return `https://github.com/${full}`
  }
  return ''
})

function displayValue(row) {
  return formatCellValue(row.kind, row.value)
}

function listValues(value) {
  if (Array.isArray(value)) return value.filter(Boolean)
  return String(value || '').split(', ').filter(Boolean)
}

const tagSuggestions = computed(() => {
  const seen = new Set(knownTags.value || [])
  for (const tag of project.value?.tags || []) {
    if (tag) seen.add(tag)
  }
  return [...seen].filter((tag) => !isHelperTag(tag)).sort((a, b) => a.localeCompare(b))
})

const annotationKeys = computed(() => {
  const keys = []
  const seen = new Set()
  for (const field of fields.value || []) {
    if (!field.id?.startsWith('annotation.')) continue
    const key = field.id.slice('annotation.'.length)
    if (!key || seen.has(key)) continue
    seen.add(key)
    keys.push(key)
  }
  for (const key of Object.keys(project.value?.annotations || {})) {
    if (key && !seen.has(key)) {
      seen.add(key)
      keys.push(key)
    }
  }
  return keys
})

function syncAnnotationDrafts() {
  if (saving.value) return
  const next = { ...annotationDrafts.value }
  let changed = false
  const live = new Set(annotationKeys.value)
  for (const key of annotationKeys.value) {
    if (activeEdit.value === key) continue
    const value = project.value?.annotations?.[key] || ''
    if (next[key] !== value) {
      next[key] = value
      changed = true
    }
  }
  for (const key of Object.keys(next)) {
    if (!live.has(key) && activeEdit.value !== key) {
      delete next[key]
      changed = true
    }
  }
  if (changed) annotationDrafts.value = next
}

function applyProject(data) {
  if (!data) return
  if (data.root) root.value = data.root
  if (data.fields) fields.value = resolveFields(data.fields)
  if (data.project) project.value = data.project
  if (Array.isArray(data.known_tags)) knownTags.value = data.known_tags
  if (data.tag_colors && typeof data.tag_colors === 'object') tagColors.value = data.tag_colors
  syncAnnotationDrafts()
}

function projectHasTag(tag) {
  return (project.value?.tags || []).includes(tag)
}

async function load() {
  if (!props.path) {
    error.value = 'Missing project path'
    loading.value = false
    project.value = null
    return
  }
  const path = props.path
  const seq = ++loadSeq
  try {
    const data = await fetchProject(path)
    if (seq !== loadSeq || path !== props.path) {
      return
    }
    applyProject(data)
    error.value = ''
    missing.value = false
  } catch (e) {
    if (seq !== loadSeq || path !== props.path) {
      return
    }
    if (isProjectNotFoundError(e)) {
      missing.value = true
      error.value = ''
    } else {
      missing.value = false
      error.value = e.message || String(e)
    }
    project.value = null
  } finally {
    if (seq === loadSeq && path === props.path) {
      loading.value = false
    }
  }
}

async function onRefresh() {
  if (!props.path || refreshing.value) return
  refreshing.value = true
  error.value = ''
  try {
    await refreshProject(props.path)
    await load()
  } catch (e) {
    error.value = e.message || String(e)
  } finally {
    refreshing.value = false
  }
}

async function toggleTag(tag) {
  if (!tag || saving.value || isHelperTag(tag)) return
  saving.value = true
  editError.value = ''
  try {
    const data = await setProjectTags({
      paths: [props.path],
      tag,
      op: projectHasTag(tag) ? 'remove' : 'add',
    })
    const updated = (data.projects || []).find((item) => item.path === props.path)
    saving.value = false
    if (!updated) {
      await load()
      return
    }
    applyProject({
      ...data,
      project: updated,
    })
  } catch (e) {
    editError.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}

async function addTag() {
  const tag = tagDraft.value.trim()
  tagDraft.value = ''
  if (!tag || projectHasTag(tag)) return
  if (isHelperTag(tag)) {
    editError.value = 'Tag is set by a helper'
    return
  }
  await toggleTag(tag)
}

async function saveAnnotation(key) {
  if (!key || saving.value) return
  const value = String(annotationDrafts.value[key] ?? '').trim()
  const current = project.value?.annotations?.[key] || ''
  if (value === current) return
  saving.value = true
  editError.value = ''
  try {
    const data = await setProjectAnnotation({ path: props.path, key, value })
    saving.value = false
    applyProject(data)
  } catch (e) {
    editError.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}

async function addAnnotation() {
  const key = newAnnotationKey.value.trim()
  if (!key || saving.value) return
  const value = newAnnotationValue.value.trim()
  saving.value = true
  editError.value = ''
  try {
    const data = await setProjectAnnotation({ path: props.path, key, value })
    newAnnotationKey.value = ''
    newAnnotationValue.value = ''
    activeEdit.value = ''
    saving.value = false
    applyProject(data)
  } catch (e) {
    editError.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}

function openEditDialog() {
  if (!project.value) return
  editError.value = ''
  editDialogRef.value?.showModal()
}

function onEditDialogBackdropClick(event) {
  if (event.target === editDialogRef.value) {
    editDialogRef.value.close()
  }
}

function openDeleteDialog() {
  if (hasLocalChanges.value || deleting.value) return
  deleteDialogRef.value?.showModal()
}

function onDeleteDialogBackdropClick(event) {
  if (event.target === deleteDialogRef.value) {
    deleteDialogRef.value.close('cancel')
  }
}

function onDeleteDialogSubmit(event) {
  if (event.submitter?.value !== 'delete') return
  event.preventDefault()
  deleteDialogRef.value?.close('delete')
  onDelete()
}

async function onDelete() {
  if (!props.path || deleting.value || hasLocalChanges.value) return
  deleting.value = true
  deleteError.value = ''
  try {
    const data = await deleteProject(props.path)
    window.dispatchEvent(new CustomEvent('plist-projects', { detail: data }))
    await router.push(backTo.value)
  } catch (e) {
    deleteError.value = e.message || String(e)
  } finally {
    deleting.value = false
  }
}

watch(project, () => {
  syncAnnotationDrafts()
})

watch(
  () => props.path,
  () => {
    loading.value = true
    load()
  },
)

onMounted(() => {
  load()
  timer = setInterval(load, 2000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <Section :subtitle="root || route.query.path" :padding="false">
    <template #title>
      <span class="project-name" :class="{ 'project-name-with-icon': iconURL }">
        <ProjectIcon v-if="iconURL" size="lg" :src="iconURL" />
        <span>{{ title }}</span>
      </span>
    </template>
    <template #toolbar>
      <template v-if="!missing">
        <button
          type="button"
          class="neutral"
          :disabled="!project"
          @click="openEditDialog"
        >Tags & annotations</button>
        <button
          type="button"
          class="neutral"
          :disabled="refreshing || !path"
          @click="onRefresh"
        >{{ refreshing ? 'Refreshing…' : 'Refresh' }}</button>
        <a
          v-if="githubURL"
          class="button"
          :href="githubURL"
          target="_blank"
          rel="noopener noreferrer"
        >GitHub</a>
      </template>
      <RouterLink :to="backTo" class="button neutral">Back</RouterLink>
    </template>

    <p v-if="loading && !project && !missing" class="scan-status padding">Loading…</p>
    <div v-else-if="missing" class="project-missing padding">
      <p>This project no longer exists. The directory may have been moved or deleted locally.</p>
      <p v-if="path" class="project-path">{{ path }}</p>
    </div>
    <p v-else-if="error && !project" class="form-error padding">{{ error }}</p>
    <div v-else-if="project" class="section-content padding">
      <p v-if="error" class="form-error">{{ error }}</p>
      <dl class="detail-list">
        <div v-for="row in detailRows" :key="row.id">
          <dt>{{ row.label }}</dt>
          <dd>
            <template v-if="row.id === 'modules' || row.id === 'tags' || row.kind === 'multi'">
              <ProjectTagList
                v-if="row.id === 'tags' && listValues(row.value).length"
                :tags="row.value"
                :tag-colors="tagColors"
                :tag-order="knownTags"
              />
              <span v-else-if="listValues(row.value).length" class="tag-list">
                <span v-for="item in listValues(row.value)" :key="item" class="tag">{{ item }}</span>
              </span>
              <span v-else>—</span>
            </template>
            <span v-else-if="row.id === 'path'" class="project-path">{{ row.value }}</span>
            <a
              v-else-if="row.id === 'github.full_name' && githubURL"
              :href="githubURL"
              target="_blank"
              rel="noopener noreferrer"
            >{{ displayValue(row) }}</a>
            <a
              v-else-if="row.id === 'github.latest_package_url' && row.value"
              :href="packageURLHref(row.value)"
              target="_blank"
              rel="noopener noreferrer"
            >{{ displayValue(row) }}</a>
            <span v-else>{{ displayValue(row) || '—' }}</span>
          </dd>
        </div>
      </dl>
    </div>
  </Section>
  <DangerZone
    v-if="project"
    description="Deletes this project's directory on disk. GitHub and other remotes are left unchanged."
  >
    <p v-if="deleteError" class="form-error">{{ deleteError }}</p>
    <p v-if="hasLocalChanges" class="form-error">
      This directory has local changes, so it cannot be deleted.
    </p>
    <button
      type="button"
      class="bad"
      :disabled="hasLocalChanges || deleting"
      @click="openDeleteDialog"
    >{{ deleting ? 'Deleting…' : 'Delete local directory' }}</button>
    <dialog
      ref="deleteDialogRef"
      class="root-dialog"
      aria-labelledby="delete-project-title"
      @click="onDeleteDialogBackdropClick"
    >
      <form method="dialog" class="root-dialog-form" @submit="onDeleteDialogSubmit">
        <h2 id="delete-project-title">Delete local directory</h2>
        <p class="subtle">
          Delete <strong>{{ title }}</strong> from disk. This cannot be undone. Nothing is changed on GitHub.
        </p>
        <fieldset>
          <button type="submit" value="cancel" class="neutral">Cancel</button>
          <button type="submit" value="delete" class="bad">Delete directory</button>
        </fieldset>
      </form>
    </dialog>
  </DangerZone>
  <dialog
    v-if="project"
    ref="editDialogRef"
    class="root-dialog edit-dialog"
    aria-labelledby="edit-project-title"
    @click="onEditDialogBackdropClick"
  >
    <div class="root-dialog-form">
      <h2 id="edit-project-title">Tags and annotations</h2>
      <div class="detail-edit">
        <div>
          <div class="detail-edit-title">Tags</div>
          <div v-if="tagSuggestions.length" class="tag-toggle-list" role="group" aria-label="Tags">
            <ProjectTag
              v-for="tag in tagSuggestions"
              :key="tag"
              :tag="tag"
              :tag-colors="tagColors"
              toggle
              :pressed="projectHasTag(tag)"
              :disabled="saving"
              @click="toggleTag(tag)"
            />
          </div>
          <p v-else class="subtle">No tags yet.</p>
          <form class="detail-edit-add" @submit.prevent="addTag">
            <input
              v-model="tagDraft"
              type="text"
              placeholder="New tag"
              aria-label="New tag"
              :disabled="saving"
              @focus="activeEdit = 'tag'"
              @blur="activeEdit = ''"
            >
            <button type="submit" class="neutral" :disabled="saving || !tagDraft.trim()">Add tag</button>
          </form>
        </div>

        <div>
          <div class="detail-edit-title">Annotations</div>
          <div v-if="annotationKeys.length" class="annotation-list">
            <label v-for="key in annotationKeys" :key="key" class="annotation-row">
              <span>@{{ key }}</span>
              <input
                v-model="annotationDrafts[key]"
                type="text"
                :aria-label="`Annotation ${key}`"
                :disabled="saving"
                placeholder="Empty clears"
                @focus="activeEdit = key"
                @blur="activeEdit = ''; saveAnnotation(key)"
                @keydown.enter.prevent="saveAnnotation(key)"
              >
            </label>
          </div>
          <p v-else class="subtle">No annotations yet.</p>
          <form class="detail-edit-add" @submit.prevent="addAnnotation">
            <input
              v-model="newAnnotationKey"
              type="text"
              placeholder="Key"
              aria-label="New annotation key"
              :disabled="saving"
              @focus="activeEdit = 'new-annotation'"
              @blur="activeEdit = ''"
            >
            <input
              v-model="newAnnotationValue"
              type="text"
              placeholder="Value"
              aria-label="New annotation value"
              :disabled="saving"
              @focus="activeEdit = 'new-annotation'"
              @blur="activeEdit = ''"
            >
            <button type="submit" class="neutral" :disabled="saving || !newAnnotationKey.trim()">Add annotation</button>
          </form>
        </div>
        <p v-if="editError" class="form-error">{{ editError }}</p>
      </div>
      <fieldset>
        <button type="button" class="neutral" @click="editDialogRef?.close()">Close</button>
      </fieldset>
    </div>
  </dialog>
</template>
