import {
  applyColumnFilter,
  filterEntrySpec,
  isFilterActive,
  sortRows,
} from 'picocrank/vue/composables/tableFilters.js'
import { normalizeFilterQuery } from 'picocrank/vue/composables/tableFilterQuery.js'
import {
  deletePreset,
  getDefaultPreset,
  getPresetById,
  listPresets,
  savePreset,
  setDefaultPreset,
} from 'picocrank/vue/composables/tableColumnPresets.js'

export const PROJECTS_TABLE_ID = 'plist-projects'

/** Split a joined multi-value cell ("Git, GitHub") into discrete tokens. */
export function splitCellTokens(value) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item).trim()).filter(Boolean)
  }
  if (value == null || value === '') {
    return []
  }
  return String(value)
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

export function collectDiscreteFilterOptions(projects, fieldID, projectFieldValue) {
  const seen = new Set()
  for (const project of projects) {
    for (const token of splitCellTokens(projectFieldValue(project, fieldID))) {
      seen.add(token)
    }
  }
  return [...seen]
    .sort((a, b) => a.localeCompare(b))
    .map((value) => ({ value, label: value }))
}

/** One option per distinct whole value. Annotation text may contain commas. */
export function collectExactFilterOptions(projects, fieldID, projectFieldValue) {
  const seen = new Set()
  for (const project of projects) {
    const value = projectFieldValue(project, fieldID)
    if (value == null || value === '') continue
    seen.add(String(value))
  }
  return [...seen]
    .sort((a, b) => a.localeCompare(b))
    .map((value) => ({ value, label: value }))
}

function rowMatchesMultiSelect(cellValue, selectedValues) {
  const tokens = new Set(splitCellTokens(cellValue))
  return selectedValues.some((selected) => tokens.has(String(selected)))
}

function applyColumnFilters(row, header, columnQuery) {
  const activeFilters = (columnQuery.filters ?? [])
    .filter((entry) => entry.enabled !== false)
    .map((entry) => filterEntrySpec(entry))
    .filter((filter) => isFilterActive(filter))

  if (activeFilters.length === 0) {
    return true
  }

  const matchFilter = (filter) => {
    if (filter.type === 'select' && header.multiValue) {
      return rowMatchesMultiSelect(row?.[header.key], filter.values)
    }
    return applyColumnFilter(row, header, filter)
  }

  if (columnQuery.match === 'any') {
    return activeFilters.some(matchFilter)
  }
  return activeFilters.every(matchFilter)
}

/** Like picocrank applyFilterQuery, but select filters on multiValue headers match tokens. */
export function applyProjectsFilterQuery(rows, headers, filterQuery) {
  const normalized = normalizeFilterQuery(filterQuery, headers, rows)
  if (normalized.columns.length === 0) {
    return rows
  }

  const headerByKey = new Map(
    headers.filter((header) => header?.key).map((header) => [header.key, header]),
  )

  return rows.filter((row) => {
    const columnResults = normalized.columns.map((column) => {
      const header = headerByKey.get(column.key)
      if (!header) {
        return true
      }
      return applyColumnFilters(row, header, column)
    })

    if (normalized.match === 'any') {
      return columnResults.some(Boolean)
    }
    return columnResults.every(Boolean)
  })
}

/** Turn per-column filter drafts into the query shape used by the projects table. */
export function buildProjectsFilterQuery(filtersByKey) {
  const columns = []
  for (const [key, partial] of Object.entries(filtersByKey || {})) {
    if (!partial || typeof partial !== 'object') continue
    columns.push({
      key,
      match: 'all',
      filters: [partial],
    })
  }
  return {
    version: 1,
    match: 'all',
    columns,
  }
}

export function createProjectsFetchRows(getRows, getHeaders) {
  return async function fetchRows(query) {
    const headers = getHeaders()
    const filtered = applyProjectsFilterQuery(getRows(), headers, query.filterQuery)
    const sorted = sortRows(filtered, query.sortBy, query.sortDir)
    const pageSize = query.pageSize || 10
    const page = query.page || 1
    const start = (page - 1) * pageSize
    return {
      rows: sorted.slice(start, start + pageSize),
      total: sorted.length,
    }
  }
}

/** Bridge picocrank localStorage presets so they keep working with fetchRows (remote mode). */
export function createLocalLayoutPresets(tableId = PROJECTS_TABLE_ID) {
  return {
    async list() {
      return listPresets(tableId)
    },
    async getDefault() {
      return { ok: true, preset: getDefaultPreset(tableId) }
    },
    async load(presetId) {
      const preset = getPresetById(tableId, presetId)
      if (!preset) {
        return { ok: false, error: 'Preset not found.' }
      }
      return { ok: true, preset }
    },
    async save({ name, state, setAsDefault }) {
      return savePreset(tableId, { name, state, setAsDefault })
    },
    async delete(presetId) {
      return deletePreset(tableId, presetId)
    },
    async setDefault(presetId) {
      return setDefaultPreset(tableId, presetId)
    },
  }
}
