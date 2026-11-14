<script setup lang="ts">
interface Address { id: number; receiver: string; phone: string; detail: string }
interface QuoteLine { sku_id: number; name: string; price_cent: number; quantity: number; discount_cent: number }
interface Quote { quote_id: string; total_cent: number; discount_cent: number; promotion_discount_cent: number; coupon_discount_cent: number; freight_cent: number; pay_cent: number; lines: QuoteLine[] }
interface PlaceReply { order: { order_no: string }; payment: { payment_no: string; amount_cent: number } }
const { request, token } = useApi()
const addresses = ref<Address[]>([])
const addressId = ref<number | null>(null)
const quote = ref<Quote | null>(null)
const message = ref('')
const loading = ref(false)
const couponId = ref('')
const router = useRouter()

async function preview(): Promise<void> {
  message.value = ''
  try {
    quote.value = await request<Quote>('/api/v1/checkout/preview', {
      method: 'POST',
      body: { address_id: addressId.value, coupon_id: couponId.value.trim() || undefined },
    })
  } catch (err) { message.value = err instanceof Error ? err.message : '结算预览失败' }
}

onMounted(async () => {
  if (!token.value) return
  try {
    const reply = await request<{ addresses: Address[] }>('/api/v1/addresses')
    addresses.value = reply.addresses ?? []
    addressId.value = addresses.value[0]?.id ?? null
    await preview()
  } catch (err) { message.value = err instanceof Error ? err.message : '结算预览失败' }
})

const clientRequestId = useState('checkout-request-id', () => newRequestId())

function newRequestId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return `web-${crypto.randomUUID()}`
  }
  return `web-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

async function place(): Promise<void> {
  if (!quote.value || !addressId.value) return
  loading.value = true
  message.value = ''
  try {
    const placed = await request<PlaceReply>('/api/v1/checkout/place', {
      method: 'POST', body: {
        quote_id: quote.value.quote_id, address_id: addressId.value,
        client_request_id: clientRequestId.value,
      },
    })
    // The payment intent amount is authoritative; do not send an amount here.
    // Mock pay is disabled in production, so a missing route means "go pay via
    // the real channel" rather than a failed checkout.
    try {
      await request(`/api/v1/payments/${placed.payment.payment_no}/pay`, { method: 'POST', body: {} })
    } catch (err) {
      const status = (err as { response?: { status?: number }; statusCode?: number })?.response?.status
        ?? (err as { statusCode?: number })?.statusCode
      if (status !== 404) throw err
    }
    clientRequestId.value = newRequestId()
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
      <div class="coupon-row">
        <van-field v-model="couponId" label="优惠券" placeholder="输入券码 / ID" />
        <van-button size="small" @click="preview">应用</van-button>
      </div>
      <template v-if="quote">
        <h3>商品</h3>
        <div v-for="line in quote.lines" :key="line.sku_id" class="line"><span>{{ line.name }} × {{ line.quantity }}</span><span>¥{{ ((line.price_cent * line.quantity - line.discount_cent) / 100).toFixed(2) }}</span></div>
        <div class="totals">
          <div><span>商品</span><span>¥{{ (quote.total_cent / 100).toFixed(2) }}</span></div>
          <div><span>促销优惠</span><span>-¥{{ ((quote.promotion_discount_cent ?? 0) / 100).toFixed(2) }}</span></div>
          <div><span>优惠券</span><span>-¥{{ ((quote.coupon_discount_cent ?? 0) / 100).toFixed(2) }}</span></div>
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
.coupon-row { display:flex; gap:8px; align-items:center; margin-top:12px; }
.line { display:flex; justify-content:space-between; padding:8px 0; border-bottom:1px solid #f1f5f9; }
.totals { display:grid; gap:6px; margin:16px 0; }
.totals div { display:flex; justify-content:space-between; }
.totals .grand { border-top:1px solid #e5e7eb; padding-top:8px; font-size:18px; font-weight:700; }
</style>
