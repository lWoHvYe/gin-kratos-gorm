<script setup lang="ts">
interface Item { sku_id: number; quantity: number }
interface AfterSale { after_sale_no: string; order_no: string; type: string; status: string; amount_cent: number; reason: string; return_carrier: string; return_tracking_no: string; refund_no: string; refunded_cent: number; items: Item[]; created_at_unix: number }
const route = useRoute()
const no = String(route.params.no)
const { request, token } = useApi()
const { data: afterSale, pending, refresh } = await useAsyncData(`after-sale-${no}`, async () => {
  if (!token.value) return null
  return await request<AfterSale>(`/api/v1/after-sales/${no}`)
})
const form = reactive({ carrier: 'SF', tracking_no: '' })
const message = ref('')
async function submitTracking(): Promise<void> {
  message.value = ''
  try {
    await request(`/api/v1/after-sales/${no}/return-tracking`, { method: 'POST', body: form })
    await refresh()
  } catch (err) { message.value = err instanceof Error ? err.message : '提交失败' }
}
</script>

<template>
  <div>
    <h1>售后详情</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <div v-else-if="pending" class="state">加载中…</div>
    <div v-else-if="!afterSale" class="panel"><van-empty description="售后不存在" /></div>
    <div v-else class="panel">
      <div class="mono">{{ afterSale.after_sale_no }}</div>
      <div class="row"><van-tag type="primary">{{ afterSale.type }}</van-tag><van-tag type="warning">{{ afterSale.status }}</van-tag></div>
      <p class="muted">订单 {{ afterSale.order_no }}</p>
      <p>原因：{{ afterSale.reason }}</p>
      <p>退款金额：<span class="price">¥{{ (afterSale.amount_cent / 100).toFixed(2) }}</span></p>
      <p v-if="afterSale.refunded_cent">已退款：¥{{ (afterSale.refunded_cent / 100).toFixed(2) }}</p>
      <p v-if="afterSale.refund_no" class="muted">退款单号：{{ afterSale.refund_no }}</p>
      <div v-if="afterSale.status === 'RETURNING'">
        <h3>填写退货物流</h3>
        <van-form @submit="submitTracking">
          <van-field v-model="form.carrier" label="承运商" />
          <van-field v-model="form.tracking_no" label="物流单号" />
          <van-button type="primary" block native-type="submit" style="margin-top:12px">提交物流</van-button>
        </van-form>
        <p v-if="message" class="muted">{{ message }}</p>
      </div>
      <div v-else-if="afterSale.return_tracking_no" class="muted">退货物流：{{ afterSale.return_carrier }} {{ afterSale.return_tracking_no }}</div>
    </div>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:16px; }
.mono { font-family: ui-monospace, monospace; font-size:12px; }
.row { display:flex; gap:8px; margin:8px 0; }
h3 { font-size:16px; margin:18px 0 8px; }
.state { padding:40px; text-align:center; color:var(--mall-muted); }
</style>
