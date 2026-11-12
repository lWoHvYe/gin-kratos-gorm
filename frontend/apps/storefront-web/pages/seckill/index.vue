<script setup lang="ts">
interface Activity { id: number; name: string; sku_id: number; price_cent: number; stock_total: number; available: number; start_at_unix: number; end_at_unix: number; status: string; server_now_unix: number }
const { request } = useApi()
const { data, pending } = await useAsyncData('seckill-activities', () => request<{ activities: Activity[]; total: number }>('/api/v1/seckill/activities?page=1&page_size=20'))
const activities = computed(() => data.value?.activities ?? [])
const now = Math.floor(Date.now() / 1000)
function label(a: Activity): string {
  if (a.end_at_unix <= now) return '已结束'
  if (a.start_at_unix > now) return '即将开始'
  return '抢购中'
}
</script>

<template>
  <div>
    <section class="hero"><h1>限时秒杀</h1><p>数量有限，先到先得。</p></section>
    <div v-if="pending" class="state">加载中…</div>
    <van-empty v-else-if="!activities.length" description="暂无秒杀活动" />
    <div v-else class="list">
      <NuxtLink v-for="a in activities" :key="a.id" :to="`/seckill/${a.id}`" class="card">
        <div class="card-head"><strong>{{ a.name }}</strong><van-tag :type="label(a) === '抢购中' ? 'primary' : 'default'">{{ label(a) }}</van-tag></div>
        <div class="muted">SKU {{ a.sku_id }} · 库存 {{ a.stock_total }}</div>
        <div class="row"><span class="price">¥{{ (a.price_cent / 100).toFixed(2) }}</span><span class="muted">{{ new Date(a.start_at_unix * 1000).toLocaleString() }}</span></div>
      </NuxtLink>
    </div>
  </div>
</template>

<style scoped>
.hero { background: linear-gradient(135deg,#dc2626,#7f1d1d); color:#fff; border-radius:14px; padding:26px 20px; margin-bottom:14px; }
.hero h1 { margin:0 0 6px; font-size:28px; }
.hero p { margin:0; opacity:.82; }
.list { display:grid; gap:12px; }
.card { background:#fff; border:1px solid #e5e7eb; border-radius:12px; padding:14px; display:grid; gap:8px; }
.card-head { display:flex; justify-content:space-between; align-items:center; gap:8px; }
.row { display:flex; justify-content:space-between; align-items:center; }
.state { text-align:center; padding:40px; color:var(--mall-muted); }
</style>
