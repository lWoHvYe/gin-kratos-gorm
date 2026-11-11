<script setup lang="ts">
interface Address { id: number; receiver: string; phone: string; detail: string }
interface QuoteLine { sku_id: number; name: string; price_cent: number; quantity: number; discount_cent: number }
interface Quote { quote_id: string; total_cent: number; discount_cent: number; promotion_discount_cent: number; freight_cent: number; pay_cent: number; lines: QuoteLine[] }
interface PlaceReply { order: { order_no: string }; payment: { payment_no: string; amount_cent: number } }
const { request, token } = useApi()
const addresses = ref<Address[]>([])
const addressId = ref<number | null>(null)
const quote = ref<Quote | null>(null)
const message = ref('')
const loading = ref(false)
const router = useRouter()

onMounted(async () => {
  if (!token.value) return
  try {
    const reply = await request<{ addresses: Address[] }>('/api/v1/addresses')
    addresses.value = reply.addresses ?? []
    addressId.value = addresses.value[0]?.id ?? null
    quote.value = await request<Quote>('/api/v1/checkout/preview', { method: 'POST', body: { address_id: addressId.value } })
  } catch (err) { message.value = err instanceof Error ? err.message : '结算预览失败' }
})

async function place(): Promise<void> {
  if (!quote.value || !addressId.value) return
  loading.value = true
  message.value = ''
  try {
    const placed = await request<PlaceReply>('/api/v1/checkout/place', {
      method: 'POST', body: {
        quote_id: quote.value.quote_id, address_id: addressId.value,
        client_request_id: `web-${Date.now()}`,
      },
    })
    await request(`/api/v1/payments/${placed.payment.payment_no}/pay`, {
      method: 'POST', body: { amount_cent: placed.payment.amount_cent },
    })
    await router.push('/orders')
  } catch (err) { message.value = err instanceof Error ? err.message : '下单失败' } finally { loading.value = false }
}
</script>

<template>
  <div>
    <h1>结算</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else class="panel">
      <div v-if="message" class="muted">{{ message }}</div>
      <div v-if="addresses.length">
        <h3>收货地址</h3>
        <van-cell v-for="address in addresses" :key="address.id" :title="`${address.receiver} ${address.phone}`" :label="address.detail" clickable @click="addressId = address.id">
          <template #right-icon><van-tag v-if="addressId === address.id" type="primary">默认</van-tag></template>
        </van-cell>
      </div>
      <div v-else class="panel-empty"><van-empty description="请先在账户中添加收货地址" /><NuxtLink to="/addresses"><van-button type="primary" block>去添加地址</van-button></NuxtLink></div>
      <template v-if="quote">
        <h3>商品</h3>
        <div v-for="line in quote.lines" :key="line.sku_id" class="line"><span>{{ line.name }} × {{ line.quantity }}</span><span>¥{{ ((line.price_cent * line.quantity - line.discount_cent) / 100).toFixed(2) }}</span></div>
        <div class="totals">
          <div><span>商品</span><span>¥{{ (quote.total_cent / 100).toFixed(2) }}</span></div>
          <div><span>促销优惠</span><span>-¥{{ (quote.promotion_discount_cent / 100).toFixed(2) }}</span></div>
          <div><span>运费</span><span>¥{{ (quote.freight_cent / 100).toFixed(2) }}</span></div>
          <div class="grand"><span>应付</span><span>¥{{ (quote.pay_cent / 100).toFixed(2) }}</span></div>
        </div>
      </template>
      <van-button type="primary" block :loading="loading" :disabled="!quote || !addressId" @click="place">提交订单并模拟支付</van-button>
    </div>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:16px; }
h3 { margin:18px 0 8px; font-size:16px; }
.line { display:flex; justify-content:space-between; padding:8px 0; border-bottom:1px solid #f1f5f9; }
.totals { display:grid; gap:6px; margin:16px 0; }
.totals div { display:flex; justify-content:space-between; }
.totals .grand { border-top:1px solid #e5e7eb; padding-top:8px; font-size:18px; font-weight:700; }
</style>
