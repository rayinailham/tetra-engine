import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Orders from './views/Orders.vue'
import Cartons from './views/Cartons.vue'
import Logs from './views/Logs.vue'
import Settings from './views/Settings.vue'

const routes = [
  { path: '/', component: Dashboard },
  { path: '/orders', component: Orders },
  { path: '/cartons', component: Cartons },
  { path: '/logs', component: Logs },
  { path: '/settings', component: Settings },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
