import { createRouter, createWebHistory } from 'vue-router'
import Login from './views/Login.vue'
import Layout from './views/Layout.vue'
import Buckets from './views/Buckets.vue'
import Objects from './views/Objects.vue'
import Users from './views/Users.vue'
import AccessKeys from './views/AccessKeys.vue'
import Policies from './views/Policies.vue'

const routes = [
  { path: '/login', component: Login, meta: { public: true } },
  {
    path: '/',
    component: Layout,
    children: [
      { path: '', redirect: '/buckets' },
      { path: 'buckets', component: Buckets },
      { path: 'buckets/:name', component: Objects },
      { path: 'users', component: Users },
      { path: 'keys', component: AccessKeys },
      { path: 'policies', component: Policies },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (to.meta.public) return
  try {
    const resp = await fetch('/api/session', { credentials: 'same-origin' })
    const data = await resp.json()
    if (!data.authenticated) {
      return '/login'
    }
  } catch {
    return '/login'
  }
})

export default router