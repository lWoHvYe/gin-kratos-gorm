<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { adminApi } from '@mall/api'
import { formatMoney } from '@mall/utils'

const products = useQuery({ queryKey: ['products', 'dashboard'], queryFn: () => adminApi.products.list({ page: 1, page_size: 5 }) })
const orders = useQuery({ queryKey: ['orders', 'dashboard'], queryFn: () => adminApi.orders.list({ page: 1, page_size: 5 }) })
const campaigns = useQuery({ queryKey: ['campaigns', 'dashboard'], queryFn: () => adminApi.campaigns.list() })
const afterSales = useQuery({ queryKey: ['aftersales', 'dashboard'], queryFn: () => adminApi.afterSales.list({ page: 1, page_size: 5 }) })

const orderTotal = computed(() => orders.data.value?.total ?? 0)
const productTotal = computed(() => products.data.value?.total ?? 0)
const activeCampaigns = computed(() => (campaigns.data.value?.campaigns ?? []).filter((c) => c.status === 'ACTIVE').length)
const pendingAfterSales = computed(() => (afterSales.data.value?.after_sales ?? []).filter((a) => a.status === 'REQUESTED').length)
const recentOrders = computed(() => orders.data.value?.orders ?? [])
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h1 class="page-title">运营总览</h1>
        <div class="page-subtitle">商品、订单、活动和售后概况</div>
      </div>
    </div>
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="panel metric"><span>商品</span><strong>{{ productTotal }}</strong><small>当前商品总数</small></div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="panel metric"><span>订单</span><strong>{{ orderTotal }}</strong><small>订单总数</small></div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="panel metric"><span>进行中活动</span><strong>{{ activeCampaigns }}</strong><small>ACTIVE 活动</small></div>
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <div class="panel metric"><span>待处理售后</span><strong>{{ pendingAfterSales }}</strong><small>REQUESTED</small></div>
      </el-col>
    </el-row>
    <el-row :gutter="16" class="section">
      <el-col :xs="24" :lg="16">
        <div class="panel">
          <div class="panel-head"><h3>最近订单</h3><router-link to="/orders">查看全部</router-link></div>
          <el-table :data="recentOrders" :loading="orders.isLoading.value" empty-text="暂无订单">
            <el-table-column prop="order_no" label="订单号" min-width="180" show-overflow-tooltip />
            <el-table-column prop="status" label="状态" width="150" />
            <el-table-column label="应付" width="110"><template #default="{ row }">{{ formatMoney(row.pay_cent) }}</template></el-table-column>
            <el-table-column prop="created_at_unix" label="创建时间" width="170">
              <template #default="{ row }">{{ new Date(row.created_at_unix * 1000).toLocaleString() }}</template>
            </el-table-column>
          </el-table>
        </div>
      </el-col>
      <el-col :xs="24" :lg="8">
        <div class="panel">
          <div class="panel-head"><h3>快捷入口</h3></div>
          <div class="quick-links">
            <router-link class="quick" to="/promotions/campaigns">新建活动</router-link>
            <router-link class="quick" to="/promotions/rules">配置促销</router-link>
            <router-link class="quick" to="/promotions/simulator">验证叠加</router-link>
            <router-link class="quick" to="/inventory">调整库存</router-link>
          </div>
          <el-alert class="tip" type="info" :closable="false" show-icon title="后台只调用 BFF API，不直接访问数据库。" />
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.section { margin-top: 16px; }
.metric { display: grid; gap: 6px; }
.metric strong { font-size: 30px; line-height: 1.1; }
.metric small { color: var(--mall-muted); }
.panel-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.panel-head h3 { margin: 0; font-size: 16px; }
.panel-head a { color: var(--mall-primary); font-size: 13px; }
.quick-links { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.quick { border: 1px solid var(--mall-border); border-radius: 8px; padding: 14px; font-weight: 600; }
.quick:hover { border-color: var(--mall-primary); color: var(--mall-primary); }
.tip { margin-top: 16px; }
</style>
