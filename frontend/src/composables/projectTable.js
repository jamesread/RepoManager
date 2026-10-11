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

/** Select filter token: annotation field is empty or missing. */
export const ANNOTATION_FILTER_UNSET = '__annotation_unset__'

/** Prefix for select filter tokens that exclude a specific annotation value. */
export const ANNOTATION_FILTER_EXCLUDE_PREFIX = '__exclude__:'

export function annotationExcludeToken(value) {
  return `${ANNOTATION_FILTER_EXCLUDE_PREFIX}${value}`
}

export function isAnnotationExcludeToken(token) {
  return String(token || '').startsWith(ANNOTATION_FILTER_EXCLUDE_PREFIX)
}

export function annotationExcludeTokenValue(token) {
  return String(token).slice(ANNOTATION_FILTER_EXCLUDE_PREFIX.length)
}

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

/** Annotation select options: unset, has value, and does-not-have value. */
export function collectAnnotationFilterOptions(projects, fieldID, projectFieldValue) {
  const options = [{ value: ANNOTATION_FILTER_UNSET, label: '(unset)' }]
  for (const opt of collectExactFilterOptions(projects, fieldID, projectFieldValue)) {
    options.push({ value: opt.value, label: opt.label })
    options.push({
      value: annotationExcludeToken(opt.value),
      label: `(not) ${opt.label}`,
    })
  }
  return options
}

export function normalizeAnnotationCellValue(value) {
  if (value == null || value === '') return ''
  return String(value).trim()
}

export function rowMatchesAnnotationSelect(cellValue, filter) {
  const selected = Array.isArray(filter?.values) ? filter.values.map(String) : []
  if (selected.length === 0) return true

  const str = normalizeAnnotationCellValue(cellValue)
  const includes = []
  const excludes = []
  let wantUnset = false

  for (const token of selected) {
    if (token === ANNOTATION_FILTER_UNSET) {
      wantUnset = true
    } else if (isAnnotationExcludeToken(token)) {
      excludes.push(annotationExcludeTokenValue(token))
    } else {
      includes.push(token)
    }
  }

  if (excludes.some((value) => value === str)) {
    return false
  }

  if (wantUnset && str === '') {
    return true
  }
  if (includes.some((value) => value === str)) {
    return true
  }
  if (includes.length === 0 && !wantUnset && excludes.length > 0) {
    return true
  }

  return includes.length === 0 && !wantUnset
}

function rowMatchesMultiSelect(cellValue, selectedValues) {
  const tokens = new Set(splitCellTokens(cellValue))
  return selectedValues.some((selected) => tokens.has(String(selected)))
}

/** Path "contains" excludes rows whose path includes the value (inverted). */
export function rowMatchesPathTextFilter(cellValue, filter) {
  if (filter?.type !== 'text' || filter.operator !== 'contains') {
    return null
  }
  const needle = String(filter.value ?? '').trim().toLowerCase()
  if (!needle) {
    return true
  }
  const haystack = cellValue == null ? '' : String(cellValue).toLowerCase()
  return !haystack.includes(needle)
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
    if (filter.type === 'select' && header.key?.startsWith('annotation.')) {
      return rowMatchesAnnotationSelect(row?.[header.key], filter)
    }
    if (filter.type === 'select' && header.multiValue) {
      return rowMatchesMultiSelect(row?.[header.key], filter.values)
    }
    if (header.key === 'path') {
      const inverted = rowMatchesPathTextFilter(row?.[header.key], filter)
      if (inverted !== null) {
        return inverted
      }
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
    const entries = Array.isArray(partial) ? partial : [partial]
    const filters = entries.filter(Boolean)
    if (filters.length === 0) continue
    columns.push({
      key,
      match: 'all',
      filters,
    })
  }
  return {
    version: 1,
    match: 'all',
    columns,
  }
}

/** Row ids currently matching active table/icon filters (not paginated). */
export function visibleFilteredProjectIds(rows, headers, filtersByKey) {
  if (!Array.isArray(rows) || rows.length === 0) {
    return new Set()
  }
  const filtered = applyProjectsFilterQuery(rows, headers, buildProjectsFilterQuery(filtersByKey))
  return new Set(filtered.map((row) => row.id).filter(Boolean))
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

/** Escape a cell for tab-separated paste into spreadsheets. */
export function escapeSpreadsheetCell(value) {
  if (value == null) {
    return ''
  }
  const text = String(value)
  if (/[\t\n\r"]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`
  }
  return text
}

/** Tab-separated table (header + rows) for the current visible columns and selection. */
export function buildProjectsSpreadsheetTSV(rows, headers, columnVisibility, selectedKeys) {
  const cols = (headers || []).filter((header) => {
    if (header.key === 'actions') {
      return false
    }
    if (columnVisibility?.[header.key] === false) {
      return false
    }
    return true
  })
  const selected = new Set(selectedKeys || [])
  const selectedRows = (rows || []).filter((row) => selected.has(row.id))
  const lines = [
    cols.map((header) => escapeSpreadsheetCell(header.label)).join('\t'),
    ...selectedRows.map((row) =>
      cols.map((header) => escapeSpreadsheetCell(row[header.key])).join('\t'),
    ),
  ]
  return lines.join('\n')
}
