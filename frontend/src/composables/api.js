/** Built-in columns matching helpers.Engine.Fields() / TUI table mode. */
export const builtinFields = [
  { id: 'name', label: 'Name', kind: 'string', sortable: true, filterable: true },
  { id: 'tags', label: 'Tags', kind: 'multi', sortable: false, filterable: true },
  { id: 'dir.mtime', label: 'Dir mtime', kind: 'time', sortable: true, filterable: true },
  { id: 'modules', label: 'Modules', kind: 'multi', sortable: false, filterable: true },
  { id: 'icon', label: 'Icon', kind: 'string', sortable: false, filterable: false },
  { id: 'git.last_commit', label: 'Git updated', kind: 'time', sortable: true, filterable: true },
  { id: 'github.owner', label: 'GitHub owner', kind: 'string', sortable: true, filterable: true },
  { id: 'github.repo', label: 'GitHub repo', kind: 'string', sortable: true, filterable: true },
  { id: 'github.full_name', label: 'GitHub full name', kind: 'string', sortable: true, filterable: true },
  { id: 'github.topics', label: 'GitHub topics', kind: 'multi', sortable: false, filterable: true },
  { id: 'github.open_issues', label: 'GitHub open issues', kind: 'int', sortable: true, filterable: true },
  { id: 'github.open_prs', label: 'GitHub open PRs', kind: 'int', sortable: true, filterable: true },
  { id: 'github.is_fork', label: 'GitHub fork', kind: 'bool', sortable: true, filterable: true },
  { id: 'github.is_archived', label: 'GitHub archived', kind: 'bool', sortable: true, filterable: true },
  { id: 'github.latest_package', label: 'GitHub latest package', kind: 'string', sortable: true, filterable: true },
  { id: 'github.latest_package_type', label: 'GitHub latest package type', kind: 'string', sortable: true, filterable: true },
  { id: 'github.latest_package_url', label: 'GitHub latest package URL', kind: 'string', sortable: true, filterable: true },
]

export const defaultHiddenFieldIDs = [
  'github.owner',
  'github.repo',
  'github.full_name',
  'github.topics',
  'github.latest_package',
  'github.latest_package_type',
  'github.latest_package_url',
]

import { Code, ConnectError } from '@connectrpc/connect'
import { client } from './client.js'

export function isProjectNotFoundError(err) {
  if (err instanceof ConnectError) {
    return err.code === Code.NotFound
  }
  return String(err?.message || err).toLowerCase().includes('not found')
}

export function structToObject(struct) {
  if (!struct) return {}
  if (struct.fields && typeof struct.fields === 'object' && !Array.isArray(struct.fields)) {
    const out = {}
    for (const [key, value] of Object.entries(struct.fields)) {
      out[key] = valueToJS(value)
    }
    return out
  }
  if (typeof struct === 'object' && !Array.isArray(struct)) {
    return { ...struct }
  }
  return {}
}

function valueToJS(value) {
  if (value == null) return null
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return value
  }
  if (Array.isArray(value)) {
    return value.map(valueToJS)
  }
  if ('stringValue' in value) return value.stringValue
  if ('numberValue' in value) return value.numberValue
  if ('boolValue' in value) return value.boolValue
  if ('listValue' in value) return value.listValue.values?.map(valueToJS) ?? []
  if ('structValue' in value) return structToObject(value.structValue)
  return null
}

function projectFromProto(p) {
  return {
    path: p.path,
    rel: p.rel,
    tags: p.tags ?? [],
    meta: structToObject(p.meta),
    annotations: { ...(p.annotations ?? {}) },
  }
}

function listBody(res) {
  return {
    root: res.root,
    scanning: res.scanning,
    found: res.found,
    fields: (res.fields ?? []).map((f) => ({
      id: f.id,
      label: f.label,
      kind: f.kind,
      sortable: f.sortable,
      filterable: f.filterable,
    })),
    projects: (res.projects ?? []).map(projectFromProto),
    recent_roots: res.recentRoots ?? [],
    known_tags: res.knownTags ?? [],
    tag_colors: { ...(res.tagColors ?? {}) },
  }
}

export async function fetchProjects() {
  const res = await client.listProjects({})
  return listBody(res)
}

export async function refreshRoot() {
  const res = await client.rescan({})
  return listBody(res)
}

export async function fetchProject(path) {
  const res = await client.getProject({ path })
  return {
    root: res.root,
    fields: (res.fields ?? []).map((f) => ({
      id: f.id,
      label: f.label,
      kind: f.kind,
      sortable: f.sortable,
      filterable: f.filterable,
    })),
    project: projectFromProto(res.project),
    known_tags: res.knownTags ?? [],
    tag_colors: { ...(res.tagColors ?? {}) },
  }
}

export async function refreshProject(path) {
  return refreshProjects([path])
}

export async function refreshProjects(paths) {
  try {
    const res = await client.refreshProjects({ paths })
    return listBody(res)
  } catch (e) {
    // Already gone from the index (e.g. deleted on disk) — caller reloads.
    if (isProjectNotFoundError(e)) {
      return { ok: true, removed: true }
    }
    throw e
  }
}

export async function deleteProject(path) {
  try {
    const res = await client.deleteProject({ path })
    return listBody(res)
  } catch (e) {
    if (String(e.message || e).includes('local changes')) {
      throw new Error('This project has local changes')
    }
    throw e
  }
}

export async function setRoot(root) {
  const res = await client.setRoot({ root })
  return listBody(res)
}

const helperTags = new Set(['changes', 'ahead', 'behind'])

export const githubTagName = 'GitHub'
export const gitTagName = 'Git'
export const maxVisibleProjectTags = 3

export function isHelperTag(tag) {
  return helperTags.has(String(tag || '').trim().toLowerCase())
}

export function isGitHubTag(tag) {
  return String(tag || '').trim() === githubTagName
}

export function isGitTag(tag) {
  return String(tag || '').trim() === gitTagName
}

export function parseTagList(tags) {
  if (Array.isArray(tags)) return tags.map((tag) => String(tag || '').trim()).filter(Boolean)
  return String(tags || '')
    .split(', ')
    .map((tag) => tag.trim())
    .filter(Boolean)
}

export function sortTagsByOrder(tags, order = []) {
  const rank = new Map()
  for (let i = 0; i < order.length; i++) {
    const tag = order[i]
    if (!rank.has(tag)) rank.set(tag, i)
  }
  return [...tags].sort((a, b) => {
    const ai = rank.has(a) ? rank.get(a) : Number.POSITIVE_INFINITY
    const bi = rank.has(b) ? rank.get(b) : Number.POSITIVE_INFINITY
    if (ai !== bi) return ai - bi
    return a.localeCompare(b, undefined, { sensitivity: 'base' })
  })
}

/** Tags for compact UI: GitHub icon is extra; Git is hidden when GitHub is present. */
export function displayProjectTags(tags, { maxVisible = maxVisibleProjectTags, tagOrder = [] } = {}) {
  const list = parseTagList(tags)
  let working = list
  if (list.some(isGitHubTag) && list.some(isGitTag)) {
    working = list.filter((tag) => !isGitTag(tag))
  }

  const github = working.filter(isGitHubTag)
  const others = working.filter((tag) => !isGitHubTag(tag))
  const sortedOthers = sortTagsByOrder(others, tagOrder)
  const visibleOthers = sortedOthers.slice(0, maxVisible)
  const hiddenCount = Math.max(0, sortedOthers.length - visibleOthers.length)

  return {
    visible: [...github, ...visibleOthers],
    hiddenCount,
  }
}

export async function setProjectTags({ paths, tag, op }) {
  const res = await client.mutateTags({ paths, tag, op })
  return listBody(res)
}

export async function setProjectAnnotation({ path, key, value }) {
  const res = await client.setAnnotation({ path, key, value })
  return {
    root: res.root,
    fields: (res.fields ?? []).map((f) => ({
      id: f.id,
      label: f.label,
      kind: f.kind,
      sortable: f.sortable,
      filterable: f.filterable,
    })),
    project: projectFromProto(res.project),
    known_tags: res.knownTags ?? [],
    tag_colors: { ...(res.tagColors ?? {}) },
  }
}

export function resolveFields(apiFields) {
  const fromAPI = Array.isArray(apiFields) ? apiFields : []
  if (fromAPI.length === 0) {
    return [...builtinFields]
  }

  const byID = new Map(builtinFields.map((f) => [f.id, { ...f }]))
  for (const f of fromAPI) {
    if (!f?.id) continue
    byID.set(f.id, {
      id: f.id,
      label: f.label || f.id,
      kind: f.kind || 'string',
      sortable: !!f.sortable,
      filterable: f.filterable !== false,
    })
  }
  return [...byID.values()]
}

export function buildDefaultColumnVisibility(headers) {
  const visibility = {}
  for (const header of headers) {
    visibility[header.key] = !defaultHiddenFieldIDs.includes(header.key)
  }
  return visibility
}

export function filterTypeForKind(kind) {
  switch (kind) {
    case 'int':
      return 'number'
    case 'bool':
      return 'boolean'
    case 'multi':
      return 'select'
    default:
      return 'text'
  }
}

export function formatCellValue(kind, value) {
  if (value == null || value === '') {
    return ''
  }
  if (Array.isArray(value)) {
    return value.join(', ')
  }
  if (kind === 'time') {
    return formatRelativeTime(value)
  }
  if (kind === 'bool') {
    return value ? 'Yes' : 'No'
  }
  return String(value)
}

export function formatRelativeTime(value) {
  const t = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(t.getTime())) {
    return String(value)
  }
  let ms = Date.now() - t.getTime()
  if (ms < 0) ms = -ms
  const minute = 60 * 1000
  const hour = 60 * minute
  const day = 24 * hour
  if (ms < minute) return 'just now'
  if (ms < hour) return `${Math.floor(ms / minute)}m ago`
  if (ms < day) return `${Math.floor(ms / hour)}h ago`
  if (ms < 30 * day) return `${Math.floor(ms / day)}d ago`
  if (ms < 365 * day) return `${Math.floor(ms / (30 * day))}mo ago`
  return `${Math.floor(ms / (365 * day))}y ago`
}

export function tagChipStyle(colors, tag) {
  const bg = colors?.[tag]
  if (!bg) return null
  return {
    backgroundColor: bg,
    borderColor: bg,
    color: '#1c1c1c',
  }
}

export function projectIconURL(projectPath, icon, rev) {
  if (!projectPath || !icon) {
    return ''
  }
  const q = new URLSearchParams({ path: projectPath, v: String(rev || icon) })
  return `/api/project/icon?${q}`
}

export function projectFieldValue(project, fieldID) {
  switch (fieldID) {
    case 'name':
      return project.rel
    case 'path':
      return project.path
    case 'tags':
      return project.tags?.length ? project.tags : null
    default:
      break
  }
  if (fieldID.startsWith('annotation.')) {
    const key = fieldID.slice('annotation.'.length)
    const v = project.annotations?.[key]
    return v ? v : null
  }
  const v = project.meta?.[fieldID]
  return v == null || v === '' ? null : v
}

/** Browser URL for a project's GitHub remote, or '' when unknown. */
export function githubProjectURL(project) {
  const meta = project?.meta || {}
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
}
