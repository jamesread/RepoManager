import { createRouter, createWebHistory } from 'vue-router'
import ProjectsIndex from '../views/ProjectsIndex.vue'
import ProjectShow from '../views/ProjectShow.vue'
import { rememberProjectView, savedProjectView } from '../composables/projectView'

const routes = [
  { path: '/', name: 'projects', component: ProjectsIndex },
  {
    path: '/project',
    name: 'project',
    component: ProjectShow,
    props: (route) => ({ path: String(route.query.path || '') }),
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
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
