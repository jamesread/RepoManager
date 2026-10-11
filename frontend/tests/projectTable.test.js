import assert from 'node:assert/strict'
import test from 'node:test'

import {
  ANNOTATION_FILTER_UNSET,
  annotationExcludeToken,
  applyProjectsFilterQuery,
  buildProjectsSpreadsheetTSV,
  escapeSpreadsheetCell,
  rowMatchesAnnotationSelect,
  rowMatchesPathTextFilter,
  visibleFilteredProjectIds,
} from '../src/composables/projectTable.js'

test('rowMatchesAnnotationSelect matches unset annotations', () => {
  assert.equal(
    rowMatchesAnnotationSelect('', { values: [ANNOTATION_FILTER_UNSET] }),
    true,
  )
  assert.equal(
    rowMatchesAnnotationSelect('motivated', { values: [ANNOTATION_FILTER_UNSET] }),
    false,
  )
})

test('rowMatchesAnnotationSelect matches included annotation values', () => {
  assert.equal(
    rowMatchesAnnotationSelect('learning', { values: ['learning'] }),
    true,
  )
  assert.equal(
    rowMatchesAnnotationSelect('archive', { values: ['learning'] }),
    false,
  )
})

test('rowMatchesAnnotationSelect excludes specific annotation values', () => {
  const filter = { values: [annotationExcludeToken('archive')] }
  assert.equal(rowMatchesAnnotationSelect('archive', filter), false)
  assert.equal(rowMatchesAnnotationSelect('learning', filter), true)
  assert.equal(rowMatchesAnnotationSelect('', filter), true)
})

test('rowMatchesPathTextFilter excludes paths containing the value', () => {
  const filter = { type: 'text', operator: 'contains', value: 'commercial' }
  assert.equal(rowMatchesPathTextFilter('/work/commercial/foo', filter), false)
  assert.equal(rowMatchesPathTextFilter('/work/personal/foo', filter), true)
  assert.equal(rowMatchesPathTextFilter('', filter), true)
})

test('applyProjectsFilterQuery inverts path contains', () => {
  const headers = [{ key: 'path', filterType: 'text' }]
  const rows = [
    { id: '/a', path: '/sandbox/commercial/acme' },
    { id: '/b', path: '/sandbox/personal/demo' },
  ]
  const filtered = applyProjectsFilterQuery(rows, headers, {
    version: 1,
    match: 'all',
    columns: [{
      key: 'path',
      match: 'all',
      filters: [{ type: 'text', operator: 'contains', value: 'commercial' }],
    }],
  })
  assert.equal(filtered.length, 1)
  assert.equal(filtered[0].id, '/b')
})

test('visibleFilteredProjectIds tracks filtered row ids', () => {
  const headers = [{ key: 'annotation.motivation', filterType: 'select' }]
  const rows = [
    { id: '/a', 'annotation.motivation': '' },
    { id: '/b', 'annotation.motivation': 'learning' },
  ]
  const visible = visibleFilteredProjectIds(rows, headers, {
    'annotation.motivation': { type: 'select', values: [ANNOTATION_FILTER_UNSET] },
  })
  assert.deepEqual([...visible], ['/a'])
})

test('applyProjectsFilterQuery applies annotation unset and exclude filters', () => {
  const headers = [{ key: 'annotation.motivation', filterType: 'select' }]
  const rows = [
    { id: 'a', 'annotation.motivation': 'learning' },
    { id: 'b', 'annotation.motivation': 'archive' },
    { id: 'c', 'annotation.motivation': '' },
  ]
  const unsetOnly = applyProjectsFilterQuery(rows, headers, {
    version: 1,
    match: 'all',
    columns: [{
      key: 'annotation.motivation',
      match: 'all',
      filters: [{ type: 'select', values: [ANNOTATION_FILTER_UNSET] }],
    }],
  })
  assert.deepEqual(unsetOnly.map((row) => row.id), ['c'])

  const notArchive = applyProjectsFilterQuery(rows, headers, {
    version: 1,
    match: 'all',
    columns: [{
      key: 'annotation.motivation',
      match: 'all',
      filters: [{ type: 'select', values: [annotationExcludeToken('archive')] }],
    }],
  })
  assert.deepEqual(notArchive.map((row) => row.id), ['a', 'c'])
})

test('buildProjectsSpreadsheetTSV exports visible columns for selected rows', () => {
  assert.equal(escapeSpreadsheetCell('plain'), 'plain')
  assert.equal(escapeSpreadsheetCell('has\ttab'), '"has\ttab"')

  const headers = [
    { key: 'name', label: 'Name' },
    { key: 'path', label: 'Path' },
    { key: 'actions', label: 'Actions' },
    { key: 'tags', label: 'Tags' },
  ]
  const rows = [
    { id: '/a', name: 'Alpha', path: '/a', tags: 'go' },
    { id: '/b', name: 'Beta', path: '/b', tags: '' },
  ]
  const tsv = buildProjectsSpreadsheetTSV(rows, headers, { path: false }, ['/b'])
  assert.equal(tsv, 'Name\tTags\nBeta\t')
})
