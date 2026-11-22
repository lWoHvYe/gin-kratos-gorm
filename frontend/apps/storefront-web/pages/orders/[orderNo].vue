<script setup lang="ts">
interface Item { sku_id: number; name: string; price_cent: number; quantity: number; discount_cent: number; warehouse_id?: number; group_no?: string }
interface FulfillmentGroup { group_no: string; warehouse_id: number; freight_cent: number; status: string }
interface OrderGroupAction { group_no: string; action: string }
interface Shipment { shipment_no: string; order_no: string; fulfillment_group_no?: string; warehouse_id?: number; status: string; carrier: string; tracking_no: string }
interface Order { order_no: string; status: string; total_cent: number; discount_cent: number; promotion_discount_cent: number; freight_cent: number; pay_cent: number; paid_cent: number; refunded_cent: number; refund_status: string; refund_state: string; available_actions: string[]; shipment_status: string; fulfillment_groups?: FulfillmentGroup[]; group_actions?: OrderGroupAction[]; receiver_name: string; receiver_phone: string; receiver_address: string; items: Item[]; created_at_unix: number; expire_at_unix: number }
const route = useRoute()
const orderNo = String(route.params.orderNo)
const { request, token } = useApi()
const message = ref('')
const { data: order, pending, refresh } = await useAsyncData(`order-${orderNo}`, async () => {
  if (!token.value) return null
  return await request<Order>(`/api/v1/orders/${orderNo}`)
})
const { data: shipments, refresh: refreshShipments } = await useAsyncData(`shipments-${orderNo}`, async () => {
  if (!token.value) return { shipments: [] as Shipment[], total: 0 }
  return await request<{ shipments: Shipment[]; total: number }>(`/api/v1/orders/${orderNo}/shipments`)
})
async function act(path: string, body: Record<string, unknown> = {}): Promise<void> {
  message.value = ''
  try {
    await request(`/api/v1/orders/${orderNo}${path}`, { method: 'POST', body })
    await refresh(); await refreshShipments()
    // Shipment -> order is an async projection; give it one short poll so the
    // per-group status and PARTIALLY_* header converge without a manual reload.
    await new Promise((resolve) => setTimeout(resolve, 700))
    await refresh(); await refreshShipments()
  } catch (err) { message.value = err instanceof Error ? err.message : '操作失败' }
}
function hasGroupAction(action: string, groupNo: string): boolean {
  if (order.value?.group_actions?.some((item) => item.action === action && item.group_no === groupNo)) return true
  // Compatibility for an older Order service during a rolling deployment.
  return order.value?.available_actions?.includes(`${action}_group:${groupNo}`) ?? false
}
function shipmentFor(groupNo: string): Shipment | undefined {
  return shipments.value?.shipments?.find((shipment) => shipment.fulfillment_group_no === groupNo)
}
async function receiveGroup(groupNo: string): Promise<void> {
  const shipment = shipmentFor(groupNo)
  if (!shipment) { message.value = '该组暂无可收货的物流单'; return }
  await act('/receive', { shipment_no: shipment.shipment_no })
}
function afterSaleLink(groupNo: string): string {
  return `/after-sales/apply?order=${orderNo}&group=${encodeURIComponent(groupNo)}`
}
</script>

<template>
  <div>
    <h1>订单详情</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else-if="pending" class="state">加载中…</div>
    <div v-else-if="!order" class="panel"><van-empty description="订单不存在" /></div>
    <div v-else class="panel">
      <div class="order-head"><span class="mono">{{ order.order_no }}</span><van-tag type="primary">{{ order.status }}</van-tag><van-tag v-if="order.shipment_status" type="success">{{ order.shipment_status }}</van-tag></div>
      <div class="muted">{{ new Date(order.created_at_unix * 1000).toLocaleString() }}</div>
      <h3>商品</h3>
      <div v-for="item in order.items" :key="item.sku_id" class="line"><span>{{ item.name }} × {{ item.quantity }}</span><span>¥{{ (((item.price_cent ?? 0) * item.quantity - (item.discount_cent ?? 0)) / 100).toFixed(2) }}</span></div>
      <div class="totals">
        <div><span>商品</span><span>¥{{ ((order.total_cent ?? 0) / 100).toFixed(2) }}</span></div>
        <div><span>促销优惠</span><span>-¥{{ ((order.promotion_discount_cent ?? 0) / 100).toFixed(2) }}</span></div>
        <div><span>总优惠</span><span>-¥{{ ((order.discount_cent ?? 0) / 100).toFixed(2) }}</span></div>
        <div><span>运费</span><span>¥{{ ((order.freight_cent ?? 0) / 100).toFixed(2) }}</span></div>
        <div class="grand"><span>应付</span><span>¥{{ ((order.pay_cent ?? 0) / 100).toFixed(2) }}</span></div>
        <div v-if="order.refunded_cent"><span>已退款</span><span>¥{{ ((order.refunded_cent ?? 0) / 100).toFixed(2) }}</span></div>
        <div v-if="order.refund_state && order.refund_state !== 'NONE'"><span>退款进度</span><span>{{ order.refund_state }}</span></div>
      </div>
      <template v-if="order.fulfillment_groups?.length">
        <h3>履约分组</h3>
        <div v-for="group in order.fulfillment_groups" :key="group.group_no" class="group">
          <div class="group-head"><strong>{{ group.group_no }}</strong><span>仓库 {{ group.warehouse_id }}</span><van-tag>{{ group.status }}</van-tag></div>
          <div class="muted">运费 ¥{{ ((group.freight_cent ?? 0) / 100).toFixed(2) }}</div>
          <div v-if="shipmentFor(group.group_no)" class="muted">{{ shipmentFor(group.group_no)?.carrier }} {{ shipmentFor(group.group_no)?.tracking_no }} · {{ shipmentFor(group.group_no)?.status }}</div>
          <div class="group-actions">
            <van-button v-if="hasGroupAction('receive', group.group_no)" size="small" type="primary" @click="receiveGroup(group.group_no)">确认收货</van-button>
            <NuxtLink v-if="hasGroupAction('apply_after_sale', group.group_no)" :to="afterSaleLink(group.group_no)"><van-button size="small" type="warning" plain>申请售后</van-button></NuxtLink>
          </div>
        </div>
      </template>
      <h3>收货信息</h3>
      <p>{{ order.receiver_name }} {{ order.receiver_phone }}</p>
      <p class="muted">{{ order.receiver_address }}</p>
      <div class="actions">
        <van-button v-if="order.available_actions?.includes('cancel')" type="danger" plain @click="act('/cancel', { reason: 'user cancelled' })">取消订单</van-button>
        <van-button v-if="order.available_actions?.includes('receive')" type="primary" @click="act('/receive')">确认收货</van-button>
        <NuxtLink v-if="order.available_actions?.includes('apply_after_sale')" :to="`/after-sales/apply?order=${order.order_no}`"><van-button type="warning" plain>申请售后</van-button></NuxtLink>
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
.group { border:1px solid var(--mall-border); border-radius:10px; padding:12px; margin:8px 0; display:grid; gap:6px; }
.group-head { display:flex; gap:10px; align-items:center; }
.group-actions { display:flex; gap:8px; flex-wrap:wrap; }
.state { padding:40px; text-align:center; color:var(--mall-muted); }
</style>
