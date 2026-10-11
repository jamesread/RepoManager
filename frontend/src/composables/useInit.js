import { reactive, readonly } from 'vue'
import { client } from './client'
import { applyAppTheming } from './applyAppTheming.js'

const state = reactive({
  ready: false,
  version: 'development',
  siteTitle: 'RepoManager',
  features: {
    themeColorSchemeSwitcherEnabled: true,
    themeName: '',
    themeControl: 'user',
    availableThemes: ['default'],
    showFooter: true,
    showNewVersions: false,
    showVersionNumber: true,
  },
  user: null,
  rbacPermissions: [],
  rbacIsSuperuser: false,
  currentVersion: '',
  availableVersion: '',
  error: null,
})

function syncFeatures(res) {
  const f = res.features || {}
  state.features = {
    themeColorSchemeSwitcherEnabled: !!f.themeColorSchemeSwitcherEnabled,
    themeName: f.themeName || '',
    themeControl: f.themeControl || 'user',
    availableThemes: f.availableThemes?.length ? [...f.availableThemes] : ['default'],
    showFooter: f.showFooter !== false,
    showNewVersions: !!f.showNewVersions,
    showVersionNumber: f.showVersionNumber !== false,
  }
}

export async function loadInit() {
  try {
    const res = await client.init({})
    state.version = res.version || 'development'
    state.siteTitle = res.siteTitle || 'RepoManager'
    syncFeatures(res)
    applyAppTheming(state.features)
    state.user = res.currentUsername ? { username: res.currentUsername } : null
    state.rbacPermissions = res.rbacPermissions?.length ? [...res.rbacPermissions] : []
    state.rbacIsSuperuser = !!res.rbacIsSuperuser
    state.currentVersion = res.currentVersion || ''
    state.availableVersion = res.availableVersion || ''
    state.error = null
  } catch (e) {
    state.error = e.message || String(e)
  } finally {
    state.ready = true
  }
}

export function setUser(user) {
  state.user = user
}

export function invalidateAppStatus() {
  state.user = null
  state.rbacPermissions = []
  state.rbacIsSuperuser = false
  state.ready = false
  state.error = null
}

export function useInit() {
  return readonly(state)
}

export { state as initState }
