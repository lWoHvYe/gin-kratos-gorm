import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('@/layouts/AdminLayout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', name: 'dashboard', component: () => import('@/views/DashboardView.vue') },
      { path: 'catalog/categories', name: 'categories', component: () => import('@/views/catalog/CategoriesView.vue') },
      { path: 'catalog/brands', name: 'brands', component: () => import('@/views/catalog/BrandsView.vue') },
      { path: 'catalog/products', name: 'products', component: () => import('@/views/catalog/ProductsView.vue') },
      { path: 'catalog/products/new', name: 'product-new', component: () => import('@/views/catalog/ProductEditView.vue') },
      { path: 'catalog/products/:id', name: 'product-edit', component: () => import('@/views/catalog/ProductEditView.vue') },
      { path: 'inventory', name: 'inventory', component: () => import('@/views/catalog/InventoryView.vue') },
      { path: 'promotions/campaigns', name: 'campaigns', component: () => import('@/views/promotions/CampaignsView.vue') },
      { path: 'promotions/rules', name: 'promotions', component: () => import('@/views/promotions/PromotionsView.vue') },
      { path: 'promotions/simulator', name: 'promotion-simulator', component: () => import('@/views/promotions/SimulatorView.vue') },
      { path: 'promotions/budget', name: 'promotion-budget', component: () => import('@/views/promotions/BudgetView.vue') },
      { path: 'orders', name: 'orders', component: () => import('@/views/orders/OrdersView.vue') },
      { path: 'orders/:orderNo', name: 'order-detail', component: () => import('@/views/orders/OrderDetailView.vue') },
      { path: 'aftersales', name: 'aftersales', component: () => import('@/views/aftersales/AfterSalesView.vue') },
      { path: 'shipments', name: 'shipments', component: () => import('@/views/orders/ShipmentsView.vue') },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (to.name === 'login' && auth.isAuthenticated && auth.isAdmin) return '/dashboard'
    return true
  }
  if (!auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (!auth.isAdmin) {
    auth.clear()
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
