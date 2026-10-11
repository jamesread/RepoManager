import assert from 'node:assert/strict'
import test from 'node:test'

import { Code, ConnectError } from '@connectrpc/connect'

import {
  displayProjectTags,
  formatCellValue,
  isGitHubTag,
  isGitTag,
  isProjectNotFoundError,
  projectFieldValue,
  structToObject,
} from '../src/composables/api.js'

test('structToObject parses Connect JSON meta', () => {
  const meta = {
    'dir.mtime': '2024-03-15T12:00:00.000000000Z',
    'github.owner': 'acme',
    modules: ['Go', 'npm'],
    'github.open_issues': 3,
  }
  assert.deepEqual(structToObject(meta), meta)
})

test('structToObject parses protobuf Struct wire meta', () => {
  const meta = {
    fields: {
      'dir.mtime': { stringValue: '2024-03-15T12:00:00.000000000Z' },
      modules: { listValue: { values: [{ stringValue: 'Go' }] } },
    },
  }
  assert.deepEqual(structToObject(meta), {
    'dir.mtime': '2024-03-15T12:00:00.000000000Z',
    modules: ['Go'],
  })
})

test('isGitHubTag matches helper tag name', () => {
  assert.equal(isGitHubTag('GitHub'), true)
  assert.equal(isGitHubTag('github'), false)
  assert.equal(isGitHubTag('Git'), false)
})

test('isGitTag matches Git helper tag name', () => {
  assert.equal(isGitTag('Git'), true)
  assert.equal(isGitTag('GitHub'), false)
})

test('displayProjectTags hides Git when GitHub is present', () => {
  const tags = ['Git', 'GitHub', 'Go', 'Rust', 'Python', 'Java']
  const { visible, hiddenCount } = displayProjectTags(tags, { tagOrder: ['Go', 'Rust', 'Python', 'Java'] })
  assert.deepEqual(visible, ['GitHub', 'Go', 'Rust', 'Python'])
  assert.equal(hiddenCount, 1)
  assert.equal(visible.includes('Git'), false)
})

test('displayProjectTags keeps Git when GitHub is absent', () => {
  const tags = ['Git', 'Go', 'Rust', 'Python']
  const { visible, hiddenCount } = displayProjectTags(tags, { tagOrder: tags })
  assert.deepEqual(visible, ['Git', 'Go', 'Rust'])
  assert.equal(hiddenCount, 1)
})

test('displayProjectTags does not count GitHub toward max visible', () => {
  const tags = ['GitHub', 'Git', 'a', 'b', 'c', 'd']
  const { visible, hiddenCount } = displayProjectTags(tags, { tagOrder: tags })
  assert.equal(visible[0], 'GitHub')
  assert.equal(visible.includes('Git'), false)
  assert.equal(visible.length, 4)
  assert.equal(hiddenCount, 1)
})

test('isProjectNotFoundError detects missing projects', () => {
  assert.equal(isProjectNotFoundError(new ConnectError('project not found', Code.NotFound)), true)
  assert.equal(isProjectNotFoundError(new Error('project not found')), true)
  assert.equal(isProjectNotFoundError(new Error('network timeout')), false)
})

test('projectFieldValue reads helper meta keys', () => {
  const project = {
    path: '/tmp/repo',
    rel: 'repo',
    tags: ['go'],
    meta: {
      'dir.mtime': '2024-03-15T12:00:00.000000000Z',
      modules: ['Go'],
    },
    annotations: {},
  }
  assert.equal(projectFieldValue(project, 'dir.mtime'), '2024-03-15T12:00:00.000000000Z')
  assert.deepEqual(projectFieldValue(project, 'modules'), ['Go'])
  assert.match(formatCellValue('time', projectFieldValue(project, 'dir.mtime')), /ago|just now/)
})
