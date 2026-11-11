<script setup lang="ts">
interface Order { order_no: string; status: string; pay_cent: number; created_at_unix: number }
const { request, token } = useApi()
const { data, pending } = await useAsyncData('orders', async () => {
  if (!token.value) return { orders: [], total: 0 }
  return await request<{ orders: Order[]; total: number }>('/api/v1/orders?page=1&page_size=20')
})
</script>

<template>
  <div>
    <h1>我的订单</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else-if="pending" class="state">加载中…</div>
    <div v-else-if="!data?.orders?.length" class="panel"><van-empty description="暂无订单" /></div>
    <div v-else class="panel">
      <div v-for="order in data?.orders ?? []" :key="order.order_no" class="order">
        <div class="mono">{{ order.order_no }}</div>
        <div class="row"><van-tag>{{ order.status }}</van-tag><span class="price">¥{{ (order.pay_cent / 100).toFixed(2) }}</span></div>
        <div class="muted">{{ new Date(order.created_at_unix * 1000).toLocaleString() }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:16px; }
.order { padding:14px 0; border-bottom:1px solid #f1f5f9; display:grid; gap:6px; }
.row { display:flex; justify-content:space-between; align-items:center; }
.mono { font-family: ui-monospace, monospace; font-size:12px; }
.state { padding:40px; text-align:center; color:var(--mall-muted); }
</style>
