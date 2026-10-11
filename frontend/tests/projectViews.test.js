import assert from 'node:assert/strict'
import test from 'node:test'

import {
  mergeProjectViewState,
  parseProjectViewState,
  serializeProjectViewState,
} from '../src/composables/projectViews.js'

test('parseProjectViewState normalizes icons layout', () => {
  const state = parseProjectViewState(JSON.stringify({
    version: 1,
    icons: {
      sortBy: 'name',
      sortDir: 'desc',
      groupBy: 'tags',
      filters: [{ key: 'tags', draft: { type: 'select', values: ['Go'] } }],
    },
  }))
  assert.equal(state.icons.sortBy, 'name')
  assert.equal(state.icons.sortDir, 'desc')
  assert.equal(state.icons.groupBy, 'tags')
  assert.equal(state.icons.filters.length, 1)
})

test('mergeProjectViewState keeps table while updating icons', () => {
  const base = parseProjectViewState(JSON.stringify({
    version: 1,
    table: { sortBy: 'path', sortDir: 'asc' },
    icons: { sortBy: 'name', sortDir: 'asc', groupBy: '', filters: [] },
  }))
  const merged = mergeProjectViewState(base, {
    icons: { sortBy: 'tags', sortDir: 'desc', groupBy: 'tags', filters: [] },
  })
  assert.equal(merged.table.sortBy, 'path')
  assert.equal(merged.icons.sortBy, 'tags')
  assert.equal(merged.icons.sortDir, 'desc')
})

test('serializeProjectViewState emits stable JSON', () => {
  const raw = serializeProjectViewState({
    icons: { sortBy: 'name', sortDir: 'asc', groupBy: '', filters: [] },
  })
  const parsed = JSON.parse(raw)
  assert.equal(parsed.version, 1)
  assert.equal(parsed.icons.sortBy, 'name')
})
