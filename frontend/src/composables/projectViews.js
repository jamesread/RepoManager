import { ConnectError } from '@connectrpc/connect'
import { sanitizePresetState } from 'picocrank/vue/composables/tableColumnPresets.js'
import { client } from './client.js'
import { loadIconViewState, sanitizeIconDraft } from './projectView.js'

export const PROJECT_VIEW_STATE_VERSION = 1
export const MAX_PROJECT_VIEWS = 20

export function emptyProjectViewState() {
  return {
    version: PROJECT_VIEW_STATE_VERSION,
    icons: null,
    table: null,
  }
}

export function sanitizeIconsViewState(state) {
  if (!state || typeof state !== 'object') return null
  const filters = []
  if (Array.isArray(state.filters)) {
    const seen = new Set()
    for (const entry of state.filters) {
      if (!entry || typeof entry.key !== 'string' || !entry.key || seen.has(entry.key)) continue
      const draft = sanitizeIconDraft(entry.draft)
      if (!draft) continue
      seen.add(entry.key)
      filters.push({ key: entry.key, draft })
    }
  }
  return {
    sortBy: typeof state.sortBy === 'string' && state.sortBy ? state.sortBy : 'name',
    sortDir: state.sortDir === 'desc' ? 'desc' : 'asc',
    groupBy: typeof state.groupBy === 'string' ? state.groupBy : '',
    filters,
  }
}

export function parseProjectViewState(raw) {
  if (!raw) return emptyProjectViewState()
  try {
    const data = typeof raw === 'string' ? JSON.parse(raw) : raw
    if (!data || typeof data !== 'object') return emptyProjectViewState()
    return {
      version: PROJECT_VIEW_STATE_VERSION,
      icons: data.icons ? sanitizeIconsViewState(data.icons) : null,
      table: data.table && typeof data.table === 'object' ? data.table : null,
    }
  } catch {
    return emptyProjectViewState()
  }
}

export function serializeProjectViewState(state) {
  const payload = emptyProjectViewState()
  if (state?.icons) {
    payload.icons = sanitizeIconsViewState(state.icons)
  }
  if (state?.table) {
    payload.table = JSON.parse(JSON.stringify(state.table))
  }
  return JSON.stringify(payload)
}

export function mergeProjectViewState(base, partial) {
  const current = parseProjectViewState(base)
  const next = {
    version: PROJECT_VIEW_STATE_VERSION,
    icons: partial?.icons != null ? sanitizeIconsViewState(partial.icons) : current.icons,
    table: partial?.table != null ? JSON.parse(JSON.stringify(partial.table)) : current.table,
  }
  return next
}

export async function listProjectViews() {
  const res = await client.listProjectViews({})
  const views = (res.views || []).map((view) => ({
    id: view.id,
    name: view.name,
    state: parseProjectViewState(view.stateJson),
    isDefault: view.isDefault,
    savedAt: view.updatedAt ? new Date(Number(view.updatedAt) * 1000).toISOString() : '',
  }))
  return {
    views,
    defaultViewId: res.defaultViewId || null,
  }
}

export async function saveProjectView({ name, state, setAsDefault = false }) {
  const res = await client.saveProjectView({
    name,
    stateJson: serializeProjectViewState(state),
    setAsDefault,
  })
  return {
    view: {
      id: res.view.id,
      name: res.view.name,
      state: parseProjectViewState(res.view.stateJson),
      isDefault: res.view.isDefault,
      savedAt: res.view.updatedAt ? new Date(Number(res.view.updatedAt) * 1000).toISOString() : '',
    },
    defaultViewId: res.defaultViewId || null,
  }
}

export async function deleteProjectView(viewId) {
  const res = await client.deleteProjectView({ viewId })
  return { defaultViewId: res.defaultViewId || null }
}

export async function setDefaultProjectView(viewId) {
  const res = await client.setDefaultProjectView({ viewId: viewId || 0 })
  return { defaultViewId: res.defaultViewId || null }
}

function presetFromView(view) {
  return {
    id: String(view.id),
    name: view.name,
    savedAt: view.savedAt,
    state: view.state?.table || {},
  }
}

/** Server-backed layout presets for the projects table (picocrank remote mode). */
export function createServerProjectViews({ getIconsState, onApplyIcons, getHeaders, getRows } = {}) {
  let cache = { views: [], defaultViewId: null }

  async function refreshCache() {
    cache = await listProjectViews()
    return cache
  }

  function resolveIconsState() {
    if (typeof getIconsState === 'function') {
      const current = getIconsState()
      if (current) return sanitizeIconsViewState(current)
    }
    return sanitizeIconsViewState(loadIconViewState())
  }

  return {
    async list() {
      const { views, defaultViewId } = await refreshCache()
      return {
        presets: views.map(presetFromView),
        defaultPresetId: defaultViewId ? String(defaultViewId) : null,
      }
    },
    async getDefault() {
      const { views, defaultViewId } = await refreshCache()
      if (!defaultViewId) {
        return { ok: true, preset: null }
      }
      const view = views.find((item) => item.id === defaultViewId)
      if (!view) {
        return { ok: true, preset: null }
      }
      if (view.state?.icons && typeof onApplyIcons === 'function') {
        onApplyIcons(view.state.icons)
      }
      return { ok: true, preset: presetFromView(view) }
    },
    async load(presetId) {
      const { views } = await refreshCache()
      const id = Number(presetId)
      const view = views.find((item) => item.id === id)
      if (!view) {
        return { ok: false, error: 'View not found.' }
      }
      if (view.state?.icons && typeof onApplyIcons === 'function') {
        onApplyIcons(view.state.icons)
      }
      return { ok: true, preset: presetFromView(view) }
    },
    async save({ name, state, setAsDefault = false }) {
      const trimmed = String(name ?? '').trim()
      if (!trimmed) {
        return { ok: false, error: 'View name is required.' }
      }
      if (!state || typeof state !== 'object') {
        return { ok: false, error: 'Nothing to save.' }
      }
      const headers = typeof getHeaders === 'function' ? getHeaders() : []
      const rows = typeof getRows === 'function' ? getRows() : []
      const table = sanitizePresetState(state, headers, rows)
      if (!table) {
        return { ok: false, error: 'Nothing to save.' }
      }
      try {
        const merged = mergeProjectViewState(emptyProjectViewState(), {
          icons: resolveIconsState(),
          table,
        })
        const result = await saveProjectView({
          name: trimmed,
          state: merged,
          setAsDefault,
        })
        await refreshCache()
        return {
          ok: true,
          preset: presetFromView(result.view),
          defaultPresetId: result.defaultViewId ? String(result.defaultViewId) : null,
        }
      } catch (err) {
        return { ok: false, error: viewErrorMessage(err) }
      }
    },
    async delete(presetId) {
      try {
        const result = await deleteProjectView(Number(presetId))
        await refreshCache()
        return {
          ok: true,
          defaultPresetId: result.defaultViewId ? String(result.defaultViewId) : null,
        }
      } catch (err) {
        return { ok: false, error: viewErrorMessage(err) }
      }
    },
    async setDefault(presetId) {
      try {
        const result = await setDefaultProjectView(presetId ? Number(presetId) : 0)
        await refreshCache()
        return {
          ok: true,
          defaultPresetId: result.defaultViewId ? String(result.defaultViewId) : null,
        }
      } catch (err) {
        return { ok: false, error: viewErrorMessage(err) }
      }
    },
  }
}

export function viewErrorMessage(err) {
  const message = err instanceof ConnectError
    ? err.message
    : (err?.rawMessage || err?.message || String(err || 'Request failed.'))
  if (message.includes('already exists')) return 'A view with that name already exists.'
  if (message.includes('limit reached') || message.includes('ResourceExhausted')) {
    return `Maximum of ${MAX_PROJECT_VIEWS} saved views reached.`
  }
  return message
}
