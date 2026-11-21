<script setup lang="ts">
import { centsFromInput } from '@mall/utils'
interface Item { sku_id: number; name: string; price_cent: number; quantity: number; warehouse_id?: number; group_no?: string }
interface Group { group_no: string; warehouse_id: number; status: string }
interface Order { order_no: string; status: string; pay_cent: number; items: Item[]; fulfillment_groups?: Group[] }
const route = useRoute()
const orderNo = String(route.query.order || '')
const selectedGroup = ref(String(route.query.group || ''))
const { request, token } = useApi()
const order = ref<Order | null>(null)
const form = reactive({ type: 'RETURN_REFUND', amount: '', reason: '' })
const quantities = reactive<Record<string, number>>({})
const groups = computed(() => order.value?.fulfillment_groups ?? [])
const visibleItems = computed(() => {
  const items = order.value?.items ?? []
  return selectedGroup.value ? items.filter((item) => (item.group_no || 'G1') === selectedGroup.value) : items
})
function itemKey(item: Item): string {
  return `${item.group_no || 'G1'}:${item.sku_id}:${item.warehouse_id || 1}`
}
function resetQuantities(): void {
  for (const key of Object.keys(quantities)) delete quantities[key]
  for (const item of visibleItems.value) quantities[itemKey(item)] = item.quantity > 0 ? 1 : 0
}
watch(selectedGroup, resetQuantities)
const message = ref('')
const loading = ref(false)
const idempotencyKey = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
  ? `as-${crypto.randomUUID()}`
  : `as-${Date.now()}-${Math.random().toString(16).slice(2)}`

onMounted(async () => {
  if (!token.value || !orderNo) return
  try {
    order.value = await request<Order>(`/api/v1/orders/${orderNo}`)
    if (!selectedGroup.value && (order.value.fulfillment_groups?.length ?? 0) > 1) {
      selectedGroup.value = order.value.fulfillment_groups?.[0]?.group_no ?? ''
    }
    resetQuantities()
  } catch (err) { message.value = err instanceof Error ? err.message : '订单加载失败' }
})

async function submit(): Promise<void> {
  if (!order.value) return
  const amountCent = centsFromInput(form.amount)
  const items = visibleItems.value
    .filter((item) => (quantities[itemKey(item)] ?? 0) > 0)
    .map((item) => ({
      sku_id: item.sku_id, quantity: quantities[itemKey(item)],
      warehouse_id: item.warehouse_id || 1, group_no: item.group_no || selectedGroup.value || 'G1',
    }))
  if (groups.value.length > 1 && !selectedGroup.value) { message.value = '请选择履约分组'; return }
  if (amountCent <= 0 || !items.length) { message.value = '请填写金额并选择退货数量'; return }
  loading.value = true
  message.value = ''
  try {
    const reply = await request<{ after_sale_no: string }>('/api/v1/after-sales', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: { order_no: order.value.order_no, type: form.type, reason: form.reason, amount_cent: amountCent, items },
    })
    await navigateTo(`/after-sales/${reply.after_sale_no}`)
  } catch (err) { message.value = err instanceof Error ? err.message : '提交失败' } finally { loading.value = false }
}
</script>

<template>
  <div>
    <h1>申请售后</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else-if="!order" class="panel"><div v-if="message" class="muted">{{ message }}</div><van-empty v-else description="订单加载中" /></div>
    <div v-else class="panel">
      <div class="muted">订单 {{ order.order_no }} · 已付 ¥{{ (order.pay_cent / 100).toFixed(2) }}</div>
      <van-form @submit="submit">
        <van-field label="类型">
          <template #input><van-tag :type="form.type === 'RETURN_REFUND' ? 'primary' : 'default'" size="large" @click="form.type = 'RETURN_REFUND'">退货退款</van-tag><van-tag :type="form.type === 'REFUND_ONLY' ? 'primary' : 'default'" size="large" style="margin-left:8px" @click="form.type = 'REFUND_ONLY'">仅退款</van-tag></template>
        </van-field>
        <van-field v-model="form.amount" label="退款金额（元）" placeholder="例如 12.34" type="number" />
        <van-field v-model="form.reason" label="原因" placeholder="请描述问题" />
        <div v-if="groups.length > 1" class="items">
          <h3>履约分组</h3>
          <van-button v-for="group in groups" :key="group.group_no" size="small" :type="selectedGroup === group.group_no ? 'primary' : 'default'" style="margin:0 8px 8px 0" @click="selectedGroup = group.group_no">{{ group.group_no }} · 仓库 {{ group.warehouse_id }}</van-button>
        </div>
        <div class="items">
          <h3>退货商品</h3>
          <div v-for="item in visibleItems" :key="itemKey(item)" class="line">
            <span>{{ item.name }}<span v-if="groups.length > 1" class="muted"> · {{ item.group_no }} / 仓库 {{ item.warehouse_id }}</span></span>
            <van-stepper v-model="quantities[itemKey(item)]" :max="item.quantity" :min="0" />
          </div>
        </div>
        <p v-if="message" class="muted">{{ message }}</p>
        <van-button type="primary" block native-type="submit" :loading="loading">提交申请</van-button>
      </van-form>
    </div>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:16px; }
h3 { font-size:16px; margin:18px 0 8px; }
.line { display:flex; justify-content:space-between; align-items:center; padding:8px 0; border-bottom:1px solid #f1f5f9; }
.items { margin:16px 0; }
</style>
