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
]

/** Columns that match the TUI table but stay off by default until enabled. */
export const defaultHiddenFieldIDs = [
  'github.owner',
  'github.repo',
  'github.full_name',
  'github.topics',
]

export async function fetchProjects() {
  const res = await fetch('/api/projects')
  if (!res.ok) {
    throw new Error(`Failed to load projects (${res.status})`)
  }
  return res.json()
}

export async function refreshRoot() {
  const res = await fetch('/api/scan', { method: 'POST' })
  if (!res.ok) {
    throw new Error(`Failed to refresh projects (${res.status})`)
  }
  return res.json()
}

export async function fetchProject(path) {
  const res = await fetch(`/api/project?path=${encodeURIComponent(path)}`)
  if (res.status === 404) {
    throw new Error('Project not found')
  }
  if (!res.ok) {
    throw new Error(`Failed to load project (${res.status})`)
  }
  return res.json()
}

export async function refreshProject(path) {
  return refreshProjects([path])
}

export async function refreshProjects(paths) {
  const res = await fetch('/api/project/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ paths }),
  })
  if (res.status === 404) {
    throw new Error('Project not found')
  }
  if (!res.ok) {
    throw new Error(`Failed to refresh project (${res.status})`)
  }
  return res.json()
}

export async function deleteProject(path) {
  const res = await fetch('/api/project/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
  if (res.status === 409) {
    throw new Error('This project has local changes')
  }
  if (res.status === 404) {
    throw new Error('Project not found')
  }
  if (!res.ok) {
    let message = `Failed to delete project (${res.status})`
    try {
      const text = (await res.text()).trim()
      if (text) message = text
    } catch {
      // keep status message
    }
    throw new Error(message)
  }
  return res.json()
}

export async function setRoot(root) {
  const res = await fetch('/api/root', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ root }),
  })
  if (!res.ok) {
    let message = `Failed to change root (${res.status})`
    try {
      const text = (await res.text()).trim()
      if (text) message = text
    } catch {
      // keep status message
    }
    throw new Error(message)
  }
  return res.json()
}

const helperTags = new Set(['changes', 'ahead', 'behind'])

/** Tags written by helpers. Users can see them, but not add or remove them. */
export function isHelperTag(tag) {
  return helperTags.has(String(tag || '').trim().toLowerCase())
}

export async function setProjectTags({ paths, tag, op }) {
  const res = await fetch('/api/tags', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ paths, tag, op }),
  })
  if (!res.ok) {
    let message = `Failed to ${op === 'remove' ? 'remove' : 'add'} tag (${res.status})`
    try {
      const text = (await res.text()).trim()
      if (text) message = text
    } catch {
      // keep status message
    }
    throw new Error(message)
  }
  return res.json()
}

export async function setProjectAnnotation({ path, key, value }) {
  const res = await fetch('/api/annotations', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path, key, value }),
  })
  if (res.status === 404) {
    throw new Error('Project not found')
  }
  if (!res.ok) {
    let message = `Failed to save annotation (${res.status})`
    try {
      const text = (await res.text()).trim()
      if (text) message = text
    } catch {
      // keep status message
    }
    throw new Error(message)
  }
  return res.json()
}

/** Prefer API field catalogue when present; always keep builtin helper fields. */
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
