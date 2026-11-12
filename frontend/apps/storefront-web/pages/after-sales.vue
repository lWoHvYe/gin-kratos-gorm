<script setup lang="ts">
interface AfterSale { after_sale_no: string; order_no: string; type: string; status: string; amount_cent: number; refunded_cent: number }
const { request, token } = useApi()
const { data, pending } = await useAsyncData('after-sales', async () => {
  if (!token.value) return { after_sales: [], total: 0 }
  return await request<{ after_sales: AfterSale[]; total: number }>('/api/v1/after-sales?page=1&page_size=20')
})
</script>

<template>
  <div>
    <h1>我的售后</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else-if="pending" class="state">加载中…</div>
    <div v-else-if="!data?.after_sales?.length" class="panel"><van-empty description="暂无售后" /></div>
    <div v-else class="panel">
      <NuxtLink v-for="item in data?.after_sales ?? []" :key="item.after_sale_no" :to="`/after-sales/${item.after_sale_no}`" class="order">
        <div class="mono">{{ item.after_sale_no }}</div>
        <div class="row"><van-tag>{{ item.type }}</van-tag><van-tag type="warning">{{ item.status }}</van-tag></div>
        <div class="muted">订单 {{ item.order_no }}</div>
        <div class="price">退款 ¥{{ (item.refunded_cent / 100).toFixed(2) }}</div>
      </NuxtLink>
    </div>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:16px; }
.order { padding:14px 0; border-bottom:1px solid #f1f5f9; display:grid; gap:6px; }
.row { display:flex; gap:8px; }
.mono { font-family: ui-monospace, monospace; font-size:12px; }
.state { padding:40px; text-align:center; color:var(--mall-muted); }
</style>
