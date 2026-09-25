const STORAGE_KEY = 'plist-projects-view'
const ICON_VIEW_KEY = 'plist-icon-view'

export function savedProjectView() {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'icons' ? 'icons' : 'table'
  } catch {
    return 'table'
  }
}

export function rememberProjectView(view) {
  try {
    localStorage.setItem(STORAGE_KEY, view === 'icons' ? 'icons' : 'table')
  } catch {
    // Storage can be unavailable in private browsing.
  }
}

function sanitizeIconDraft(draft) {
  if (!draft || typeof draft !== 'object') return null
  if (draft.type === 'select') {
    const values = Array.isArray(draft.values) ? draft.values.map((value) => String(value)) : []
    return { type: 'select', values }
  }
  if (draft.type === 'number') {
    return {
      type: 'number',
      operator: typeof draft.operator === 'string' ? draft.operator : 'eq',
      value: draft.value == null ? '' : String(draft.value),
    }
  }
  if (draft.type === 'text') {
    return {
      type: 'text',
      operator: typeof draft.operator === 'string' ? draft.operator : 'contains',
      value: draft.value == null ? '' : String(draft.value),
    }
  }
  return null
}

/** Sort, group, and filters for the big-icon project view. */
export function loadIconViewState() {
  try {
    const raw = localStorage.getItem(ICON_VIEW_KEY)
    if (!raw) return null
    const data = JSON.parse(raw)
    if (!data || typeof data !== 'object') return null
    const filters = []
    if (Array.isArray(data.filters)) {
      const seen = new Set()
      for (const entry of data.filters) {
        if (!entry || typeof entry.key !== 'string' || !entry.key || seen.has(entry.key)) continue
        const draft = sanitizeIconDraft(entry.draft)
        if (!draft) continue
        seen.add(entry.key)
        filters.push({ key: entry.key, draft })
      }
    }
    return {
      sortBy: typeof data.sortBy === 'string' && data.sortBy ? data.sortBy : 'name',
      sortDir: data.sortDir === 'desc' ? 'desc' : 'asc',
      groupBy: typeof data.groupBy === 'string' ? data.groupBy : '',
      filters,
    }
  } catch {
    return null
  }
}

export function saveIconViewState(state) {
  try {
    localStorage.setItem(ICON_VIEW_KEY, JSON.stringify({
      sortBy: state.sortBy,
      sortDir: state.sortDir === 'desc' ? 'desc' : 'asc',
      groupBy: state.groupBy || '',
      filters: (state.filters || []).map((entry) => ({
        key: entry.key,
        draft: sanitizeIconDraft(entry.draft),
      })).filter((entry) => entry.key && entry.draft),
    }))
  } catch {
    // Storage can be unavailable in private browsing.
  }
}

export function projectsListLocation() {
  if (savedProjectView() === 'icons') {
    return { name: 'projects', query: { view: 'icons' } }
  }
  return { name: 'projects', query: { view: 'table' } }
}
