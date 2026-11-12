<script setup lang="ts">
interface Activity { id: number; name: string; sku_id: number; price_cent: number; stock_total: number; available: number; start_at_unix: number; end_at_unix: number; status: string; server_now_unix: number }
interface Address { id: number; receiver: string; phone: string; province: string; city: string; district: string; detail: string }
interface Qualification { token: string; activity_id: number; user_id: number; status: string; order_no: string; payment_no: string; expires_at_unix: number }
const route = useRoute()
const { request, token } = useApi()
const activityID = Number(route.params.id)
const { data: activity, pending } = await useAsyncData(`seckill-${activityID}`, () => request<Activity>(`/api/v1/seckill/activities/${activityID}`))
const addresses = ref<Address[]>([])
const addressID = ref<number | null>(null)
const qualification = ref<Qualification | null>(null)
const message = ref('')
const loading = ref(false)
const offset = ref(0)
const now = ref(Math.floor(Date.now() / 1000))
let timer: ReturnType<typeof setInterval> | null = null

const phase = computed(() => {
  if (!activity.value) return 'LOADING'
  if (now.value >= activity.value.end_at_unix) return 'ENDED'
  if (now.value < activity.value.start_at_unix) return 'UPCOMING'
  if (activity.value.available <= 0) return 'SOLD_OUT'
  return 'ACTIVE'
})
const countdown = computed(() => {
  if (!activity.value) return ''
  const target = phase.value === 'UPCOMING' ? activity.value.start_at_unix : activity.value.end_at_unix
  const seconds = Math.max(target - now.value, 0)
  const h = Math.floor(seconds / 3600); const m = Math.floor((seconds % 3600) / 60); const s = seconds % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
})

onMounted(async () => {
  if (activity.value) offset.value = activity.value.server_now_unix - Math.floor(Date.now() / 1000)
  now.value = Math.floor(Date.now() / 1000) + offset.value
  timer = setInterval(() => { now.value = Math.floor(Date.now() / 1000) + offset.value }, 1000)
  if (token.value) {
    try {
      const reply = await request<{ addresses: Address[] }>('/api/v1/addresses')
      addresses.value = reply.addresses ?? []
      addressID.value = addresses.value[0]?.id ?? null
    } catch { /* address is optional until reserve */ }
  }
})
onUnmounted(() => { if (timer) clearInterval(timer) })

async function reserve(): Promise<void> {
  message.value = ''
  if (!token.value) { await navigateTo(`/login?redirect=/seckill/${activityID}`); return }
  const address = addresses.value.find((a) => a.id === addressID.value)
  if (!address) { message.value = '请先添加收货地址'; return }
  loading.value = true
  try {
    const q = await request<Qualification>(`/api/v1/seckill/activities/${activityID}/reserve`, {
      method: 'POST', body: {
        address_id: address.id, receiver_name: address.receiver, receiver_phone: address.phone,
        receiver_address: `${address.province}${address.city}${address.district}${address.detail}`,
      },
    })
    qualification.value = q
    for (let i = 0; i < 20; i++) {
      if (q.status === 'ORDERED' || q.status === 'FAILED' || q.status === 'EXPIRED' || q.status === 'CANCELLED') break
      await new Promise((resolve) => setTimeout(resolve, 1000))
      const latest = await request<Qualification>(`/api/v1/seckill/qualifications/${q.token}`)
      qualification.value = latest
    }
    if (activity.value) activity.value.available = Math.max(activity.value.available - 1, 0)
  } catch (err) { message.value = err instanceof Error ? err.message : '抢购失败' } finally { loading.value = false }
}

async function pay(): Promise<void> {
  if (!qualification.value?.payment_no) return
  loading.value = true
  try {
    await request(`/api/v1/payments/${qualification.value.payment_no}/pay`, { method: 'POST', body: { amount_cent: activity.value?.price_cent ?? 0 } })
    await navigateTo(`/orders/${qualification.value.order_no}`)
  } catch (err) { message.value = err instanceof Error ? err.message : '支付失败' } finally { loading.value = false }
}
</script>

<template>
  <div v-if="pending" class="state">加载中…</div>
  <van-empty v-else-if="!activity" description="活动不存在" />
  <div v-else class="panel">
    <div class="head"><div><h1>{{ activity.name }}</h1><p class="muted">SKU {{ activity.sku_id }}</p></div><van-tag type="danger">{{ phase }}</van-tag></div>
    <div class="countdown">{{ phase === 'UPCOMING' ? '距开始' : '距结束' }} <strong>{{ countdown }}</strong></div>
    <div class="price">¥{{ (activity.price_cent / 100).toFixed(2) }}</div>
    <div class="muted">剩余库存：{{ activity.available }} / {{ activity.stock_total }}</div>
    <template v-if="!token">
      <NuxtLink :to="`/login?redirect=/seckill/${activityID}`"><van-button type="primary" block>登录后抢购</van-button></NuxtLink>
    </template>
    <template v-else-if="phase === 'ACTIVE'">
      <div v-if="addresses.length" class="addresses">
        <h3>收货地址</h3>
        <van-cell v-for="a in addresses" :key="a.id" :title="`${a.receiver} ${a.phone}`" :label="`${a.province}${a.city}${a.district}${a.detail}`" clickable @click="addressID = a.id">
          <template #right-icon><van-tag v-if="addressID === a.id" type="primary">使用</van-tag></template>
        </van-cell>
      </div>
      <div v-else class="muted">请先 <NuxtLink to="/addresses">添加收货地址</NuxtLink></div>
      <van-button type="danger" block :loading="loading" :disabled="!addressID" @click="reserve">立即抢购</van-button>
    </template>
    <van-empty v-else :description="phase === 'UPCOMING' ? '活动尚未开始' : phase === 'ENDED' ? '活动已结束' : '已售罄'" />

    <div v-if="qualification" class="result">
      <h3>抢购结果</h3>
      <p>状态：<strong>{{ qualification.status }}</strong></p>
      <template v-if="qualification.status === 'ORDERED'">
        <p>订单：{{ qualification.order_no }}</p>
        <NuxtLink v-if="qualification.payment_no" :to="`/orders/${qualification.order_no}`"><van-button type="primary" block :loading="loading" @click="pay">支付并查看订单</van-button></NuxtLink>
      </template>
      <p v-else-if="qualification.status === 'ISSUED' || qualification.status === 'ORDERING'" class="muted">排队中，请稍候…</p>
    </div>
    <p v-if="message" class="muted">{{ message }}</p>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:18px; }
.head { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; }
h1 { margin:0 0 4px; font-size:24px; }
.countdown { margin:16px 0; font-size:18px; }
.countdown strong { color:#dc2626; font-size:26px; margin-left:6px; }
.price { color:#dc2626; font-size:34px; font-weight:800; margin:10px 0; }
.addresses { margin:16px 0; }
h3 { font-size:16px; margin:16px 0 8px; }
.result { border-top:1px solid #e5e7eb; margin-top:18px; padding-top:14px; }
.state { padding:40px; text-align:center; color:var(--mall-muted); }
</style>
