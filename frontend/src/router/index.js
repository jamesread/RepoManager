import { createRouter, createWebHistory } from 'vue-router'
import ProjectsIndex from '../views/ProjectsIndex.vue'
import ProjectShow from '../views/ProjectShow.vue'
import Login from '../views/Login.vue'
import SettingsAdmin from '../views/SettingsAdmin.vue'
import SystemControlPanel from '../views/SystemControlPanel.vue'
import DatabaseAdmin from '../views/DatabaseAdmin.vue'
import Iam from '../views/Iam.vue'
import UsersAdmin from '../views/UsersAdmin.vue'
import MyPermissions from '../views/MyPermissions.vue'
import UserControlPanel from '../views/UserControlPanel.vue'
import ChangePassword from '../views/ChangePassword.vue'
import { rememberProjectView, savedProjectView } from '../composables/projectView'
import { initState, loadInit } from '../composables/useInit.js'
import {
  canAccessControlPanel,
  canAccessDatabase,
  canAccessIam,
  canAccessSettings,
} from '../composables/rbacAccess.js'

const routes = [
  { path: '/login', name: 'login', component: Login, meta: { public: true } },
  { path: '/', name: 'projects', component: ProjectsIndex },
  {
    path: '/project',
    name: 'project',
    component: ProjectShow,
    props: (route) => ({ path: String(route.query.path || '') }),
  },
  {
    path: '/user-control-panel',
    name: 'userControlPanel',
    component: UserControlPanel,
  },
  {
    path: '/user-control-panel/permissions',
    name: 'user-my-permissions',
    component: MyPermissions,
  },
  { path: '/change-password', name: 'change-password', component: ChangePassword },
  {
    path: '/system',
    name: 'control-panel',
    component: SystemControlPanel,
    meta: { requiresControlPanel: true },
  },
  { path: '/system/settings', name: 'settings', component: SettingsAdmin, meta: { requiresSettings: true } },
  {
    path: '/system/database',
    name: 'system-database',
    component: DatabaseAdmin,
    meta: { requiresDatabase: true },
  },
  {
    path: '/system/iam',
    component: Iam,
    meta: { requiresIam: true },
    children: [
      { path: '', redirect: { name: 'iam-users' } },
      { path: 'users', name: 'iam-users', component: UsersAdmin },
      { path: 'my-permissions', name: 'my-permissions', component: MyPermissions },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (!initState.ready) {
    await loadInit()
  }
  if (to.meta.public) {
    return true
  }
  if (!initState.user) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.requiresControlPanel && !canAccessControlPanel()) {
    return { name: 'projects' }
  }
  if (to.meta.requiresSettings && !canAccessSettings()) {
    return { name: 'projects' }
  }
  if (to.meta.requiresDatabase && !canAccessDatabase()) {
    return { name: 'projects' }
  }
  if (to.meta.requiresIam && !canAccessIam()) {
    return { name: 'projects' }
  }
  if (to.path.startsWith('/system') && !canAccessControlPanel()) {
    return { name: 'projects' }
  }
  if (to.name !== 'projects') return true
  if (to.query.view === 'icons' || to.query.view === 'table') {
    rememberProjectView(to.query.view)
    return true
  }
  const saved = savedProjectView()
  return {
    name: 'projects',
    query: { ...to.query, view: saved },
    replace: true,
  }
})

export default router
