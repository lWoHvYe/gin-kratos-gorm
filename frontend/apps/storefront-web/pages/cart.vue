<script setup lang="ts">
interface CartItem { sku_id: number; name: string; price_cent: number; quantity: number; selected: boolean }
interface Cart { items: CartItem[]; total_cent: number }
const { request, token } = useApi()
const { data, pending, refresh } = await useAsyncData('cart', async () => {
  if (!token.value) return { items: [], total_cent: 0 } satisfies Cart
  return await request<Cart>('/api/v1/cart')
})
async function remove(skuId: number): Promise<void> {
  await request('/api/v1/cart/items', { method: 'DELETE', body: { sku_id: skuId } })
  await refresh()
}
</script>

<template>
  <div>
    <h1>购物车</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else-if="pending" class="state">加载中…</div>
    <div v-else-if="!data?.items?.length" class="panel"><van-empty description="购物车是空的" /></div>
    <div v-else class="panel">
      <div v-for="item in data?.items ?? []" :key="item.sku_id" class="line">
        <div><div class="name">{{ item.name }}</div><div class="muted">x{{ item.quantity }}</div></div>
        <div class="right"><span class="price">¥{{ ((item.price_cent * item.quantity) / 100).toFixed(2) }}</span><van-button size="small" plain type="danger" @click="remove(item.sku_id)">移除</van-button></div>
      </div>
      <div class="total">合计 <span class="price">¥{{ ((data?.total_cent ?? 0) / 100).toFixed(2) }}</span></div>
      <NuxtLink to="/checkout"><van-button type="primary" block style="margin-top:12px">去结算</van-button></NuxtLink>
    </div>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:16px; }
.line { display:flex; justify-content:space-between; align-items:center; padding:12px 0; border-bottom:1px solid #f1f5f9; }
.right { display:flex; align-items:center; gap:10px; }
.total { display:flex; justify-content:space-between; padding-top:16px; font-size:18px; }
.state { padding:40px; text-align:center; color:var(--mall-muted); }
</style>
