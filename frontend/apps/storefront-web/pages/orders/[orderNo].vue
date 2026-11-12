<script setup lang="ts">
interface Item { sku_id: number; name: string; price_cent: number; quantity: number; discount_cent: number }
interface Order { order_no: string; status: string; total_cent: number; discount_cent: number; promotion_discount_cent: number; freight_cent: number; pay_cent: number; paid_cent: number; refunded_cent: number; refund_status: string; receiver_name: string; receiver_phone: string; receiver_address: string; items: Item[]; created_at_unix: number; expire_at_unix: number }
const route = useRoute()
const orderNo = String(route.params.orderNo)
const { request, token } = useApi()
const message = ref('')
const { data: order, pending, refresh } = await useAsyncData(`order-${orderNo}`, async () => {
  if (!token.value) return null
  return await request<Order>(`/api/v1/orders/${orderNo}`)
})
async function act(path: string, body: Record<string, unknown> = {}): Promise<void> {
  message.value = ''
  try { await request(`/api/v1/orders/${orderNo}${path}`, { method: 'POST', body }); await refresh() } catch (err) { message.value = err instanceof Error ? err.message : '操作失败' }
}
</script>

<template>
  <div>
    <h1>订单详情</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else-if="pending" class="state">加载中…</div>
    <div v-else-if="!order" class="panel"><van-empty description="订单不存在" /></div>
    <div v-else class="panel">
      <div class="order-head"><span class="mono">{{ order.order_no }}</span><van-tag type="primary">{{ order.status }}</van-tag></div>
      <div class="muted">{{ new Date(order.created_at_unix * 1000).toLocaleString() }}</div>
      <h3>商品</h3>
      <div v-for="item in order.items" :key="item.sku_id" class="line"><span>{{ item.name }} × {{ item.quantity }}</span><span>¥{{ ((item.price_cent * item.quantity - item.discount_cent) / 100).toFixed(2) }}</span></div>
      <div class="totals">
        <div><span>商品</span><span>¥{{ (order.total_cent / 100).toFixed(2) }}</span></div>
        <div><span>促销优惠</span><span>-¥{{ (order.promotion_discount_cent / 100).toFixed(2) }}</span></div>
        <div><span>总优惠</span><span>-¥{{ (order.discount_cent / 100).toFixed(2) }}</span></div>
        <div><span>运费</span><span>¥{{ (order.freight_cent / 100).toFixed(2) }}</span></div>
        <div class="grand"><span>应付</span><span>¥{{ (order.pay_cent / 100).toFixed(2) }}</span></div>
        <div v-if="order.refunded_cent"><span>已退款</span><span>¥{{ (order.refunded_cent / 100).toFixed(2) }}</span></div>
      </div>
      <h3>收货信息</h3>
      <p>{{ order.receiver_name }} {{ order.receiver_phone }}</p>
      <p class="muted">{{ order.receiver_address }}</p>
      <div class="actions">
        <van-button v-if="order.status === 'PENDING_PAYMENT'" type="danger" plain @click="act('/cancel', { reason: 'user cancelled' })">取消订单</van-button>
        <van-button v-if="order.status === 'SHIPPED'" type="primary" @click="act('/receive')">确认收货</van-button>
        <NuxtLink v-if="['PAID','ALLOCATED','SHIPPED','COMPLETED','PARTIALLY_REFUNDED'].includes(order.status)" :to="`/after-sales/apply?order=${order.order_no}`"><van-button type="warning" plain>申请售后</van-button></NuxtLink>
      </div>
      <p v-if="message" class="muted">{{ message }}</p>
    </div>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:16px; }
.order-head { display:flex; justify-content:space-between; align-items:center; gap:8px; margin-bottom:4px; }
.mono { font-family: ui-monospace, monospace; font-size:12px; }
h3 { font-size:16px; margin:18px 0 8px; }
.line { display:flex; justify-content:space-between; padding:8px 0; border-bottom:1px solid #f1f5f9; }
.totals { display:grid; gap:6px; margin:16px 0; }
.totals div { display:flex; justify-content:space-between; }
.totals .grand { border-top:1px solid #e5e7eb; padding-top:8px; font-size:18px; font-weight:700; }
.actions { display:flex; gap:10px; flex-wrap:wrap; margin-top:16px; }
.state { padding:40px; text-align:center; color:var(--mall-muted); }
</style>
