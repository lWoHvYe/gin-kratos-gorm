<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const active = computed(() => route.path)

async function logout(): Promise<void> {
  try {
    await ElMessageBox.confirm('确定退出后台吗？', '退出登录', { type: 'warning' })
  } catch {
    return
  }
  await auth.logout()
  await router.replace('/login')
}
</script>

<template>
  <el-container class="admin-shell">
    <el-aside width="232px" class="sidebar">
      <div class="logo">
        <span class="logo-mark">M</span>
        <span>Mall Admin</span>
      </div>
      <el-menu :default-active="active" router class="menu">
        <el-menu-item index="/dashboard">总览</el-menu-item>
        <el-sub-menu index="catalog">
          <template #title>商品中心</template>
          <el-menu-item index="/catalog/categories">分类</el-menu-item>
          <el-menu-item index="/catalog/brands">品牌</el-menu-item>
          <el-menu-item index="/catalog/products">商品 / SKU</el-menu-item>
          <el-menu-item index="/inventory">库存</el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="promotions">
          <template #title>促销运营</template>
          <el-menu-item index="/promotions/campaigns">活动</el-menu-item>
          <el-menu-item index="/promotions/rules">促销规则</el-menu-item>
          <el-menu-item index="/promotions/simulator">促销模拟器</el-menu-item>
        </el-sub-menu>
        <el-menu-item index="/orders">订单</el-menu-item>
        <el-menu-item index="/shipments">发货</el-menu-item>
        <el-menu-item index="/aftersales">售后</el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <div>
          <div class="header-title">{{ route.meta.title || '运营后台' }}</div>
          <div class="muted">内部运营工作台</div>
        </div>
        <div class="header-right">
          <span class="muted">{{ auth.user?.nickname || auth.user?.username }}</span>
          <el-button text type="primary" @click="logout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.admin-shell { min-height: 100vh; }
.sidebar { background: #0f172a; color: #e2e8f0; }
.logo { height: 64px; display: flex; align-items: center; gap: 10px; padding: 0 20px; font-weight: 700; letter-spacing: 0.2px; }
.logo-mark { width: 28px; height: 28px; border-radius: 8px; display: grid; place-items: center; background: var(--mall-primary); color: #fff; }
.menu { border-right: 0; background: transparent; }
:deep(.el-menu-item), :deep(.el-sub-menu__title) { color: #cbd5e1; }
:deep(.el-menu-item.is-active) { color: #fff; background: rgba(37, 99, 235, 0.35); }
:deep(.el-menu-item:hover), :deep(.el-sub-menu__title:hover) { background: rgba(148, 163, 184, 0.12); color: #fff; }
.header { height: 68px; background: #fff; border-bottom: 1px solid var(--mall-border); display: flex; align-items: center; justify-content: space-between; }
.header-title { font-weight: 700; }
.header-right { display: flex; align-items: center; gap: 10px; }
.main { padding: 20px; }
</style>
