<script setup lang="ts">
import { centsFromInput } from '@mall/utils'
interface Item { sku_id: number; name: string; price_cent: number; quantity: number }
interface Order { order_no: string; status: string; pay_cent: number; items: Item[] }
const route = useRoute()
const orderNo = String(route.query.order || '')
const { request, token } = useApi()
const order = ref<Order | null>(null)
const form = reactive({ type: 'RETURN_REFUND', amount: '', reason: '' })
const quantities = reactive<Record<number, number>>({})
const message = ref('')
const loading = ref(false)
const idempotencyKey = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
  ? `as-${crypto.randomUUID()}`
  : `as-${Date.now()}-${Math.random().toString(16).slice(2)}`

onMounted(async () => {
  if (!token.value || !orderNo) return
  try {
    order.value = await request<Order>(`/api/v1/orders/${orderNo}`)
    for (const item of order.value.items) quantities[item.sku_id] = item.quantity > 0 ? 1 : 0
  } catch (err) { message.value = err instanceof Error ? err.message : '订单加载失败' }
})

async function submit(): Promise<void> {
  if (!order.value) return
  const amountCent = centsFromInput(form.amount)
  const items = Object.entries(quantities).filter(([, qty]) => qty > 0).map(([sku, qty]) => ({ sku_id: Number(sku), quantity: qty }))
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
        <div class="items">
          <h3>退货商品</h3>
          <div v-for="item in order.items" :key="item.sku_id" class="line">
            <span>{{ item.name }}</span>
            <van-stepper v-model="quantities[item.sku_id]" :max="item.quantity" :min="0" />
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
