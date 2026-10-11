import { initState } from './useInit.js'

export function hasPermission(name) {
  if (initState.rbacIsSuperuser) return true
  return initState.rbacPermissions.includes(name)
}

export function canAccessIam() {
  return hasPermission('users.view') || hasPermission('usergroups.view') || hasPermission('rbac.view')
}

export function canAccessSettings() {
  return hasPermission('system.settings')
}

export function canAccessDatabase() {
  return hasPermission('system.logs')
}

export function canAccessControlPanel() {
  return canAccessIam() || canAccessSettings() || canAccessDatabase()
}
