<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { adminApi, ApiError, type FeedbackSummary } from '@mall/api'

const days = ref(7)
const summary = ref<FeedbackSummary | null>(null)
const loading = ref(false)
const error = ref('')
const brief = ref<Awaited<ReturnType<typeof adminApi.ai.opsBrief>> | null>(null)
const briefLoading = ref(false)
const briefFeedback = ref(0)

const helpfulRate = computed(() => {
  const total = summary.value?.total ?? 0
  if (!total) return '—'
  return `${Math.round(((summary.value?.helpful ?? 0) / total) * 100)}%`
})

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    summary.value = await adminApi.ai.feedbackSummary(days.value)
  } catch (err) {
    error.value = err instanceof ApiError || err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function rateBrief(rating: 1 | -1): Promise<void> {
  if (!brief.value || briefFeedback.value !== 0) return
  try {
    await adminApi.ai.opsFeedback({ rating, day: new Date(brief.value.generated_at * 1000).toISOString().slice(0, 10) })
    briefFeedback.value = rating
  } catch {
    // Feedback is best-effort.
  }
}

async function loadBrief(latest = false): Promise<void> {
  briefLoading.value = true
  try {
    brief.value = latest ? await adminApi.ai.latestOpsBrief() : await adminApi.ai.opsBrief(days.value)
  } catch (err) {
    error.value = err instanceof ApiError || err instanceof Error ? err.message : '简报加载失败'
  } finally {
    briefLoading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div><h1 class="page-title">AI 质量</h1><div class="page-subtitle">基于 storefront 明确反馈；只统计用户主动评价</div></div>
      <div class="filters">
        <el-select v-model="days" style="width:120px" @change="load">
          <el-option :value="1" label="近 1 天" />
          <el-option :value="7" label="近 7 天" />
          <el-option :value="30" label="近 30 天" />
        </el-select>
        <el-button :loading="loading" @click="load">刷新</el-button>
        <el-button :loading="briefLoading" @click="loadBrief()">运营简报</el-button>
        <el-button :loading="briefLoading" @click="loadBrief(true)">最新简报</el-button>
      </div>
    </div>
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-row :gutter="16">
      <el-col :xs="24" :sm="12" :lg="6"><div class="panel metric"><span>反馈总数</span><strong>{{ summary?.total ?? 0 }}</strong></div></el-col>
      <el-col :xs="24" :sm="12" :lg="6"><div class="panel metric"><span>有帮助</span><strong>{{ summary?.helpful ?? 0 }}</strong></div></el-col>
      <el-col :xs="24" :sm="12" :lg="6"><div class="panel metric"><span>没帮助</span><strong>{{ summary?.not_helpful ?? 0 }}</strong></div></el-col>
      <el-col :xs="24" :sm="12" :lg="6"><div class="panel metric"><span>有帮助率</span><strong>{{ helpfulRate }}</strong></div></el-col>
    </el-row>
    <div v-if="brief" class="panel section">
      <h3>运营简报</h3>
      <p>{{ brief.summary }}</p>
      <p class="muted">订单 {{ brief.total_orders }} · 已支付 {{ brief.paid_orders }} · 售后 {{ brief.after_sales }} · 低库存 {{ brief.low_stock }}</p>
      <ul v-if="brief.exception_links?.length" class="links">
        <li v-for="link in brief.exception_links" :key="link"><a :href="link">{{ link }}</a></li>
      </ul>
      <div class="brief-feedback">
        <span>这份简报有帮助吗？</span>
        <el-button size="small" :disabled="briefFeedback !== 0" @click="rateBrief(1)">👍</el-button>
        <el-button size="small" :disabled="briefFeedback !== 0" @click="rateBrief(-1)">👎</el-button>
        <span v-if="briefFeedback !== 0">已记录</span>
      </div>
    </div>
    <el-row :gutter="16" class="section">
      <el-col :xs="24" :lg="14">
        <div class="panel">
          <h3>按天</h3>
          <el-table :data="summary?.days ?? []" :loading="loading" empty-text="近几天没有反馈">
            <el-table-column prop="day" label="日期" min-width="130" />
            <el-table-column prop="helpful" label="有帮助" width="110" />
            <el-table-column prop="not_helpful" label="没帮助" width="110" />
          </el-table>
        </div>
      </el-col>
      <el-col :xs="24" :lg="10">
        <div class="panel">
          <h3>没帮助的留言</h3>
          <el-empty v-if="!(summary?.not_helpful_comments ?? []).length" description="暂无留言" />
          <ul v-else class="comments">
            <li v-for="(item, i) in summary?.not_helpful_comments" :key="i"><span>{{ item.day }}</span>{{ item.comment }}</li>
          </ul>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.filters { display: flex; gap: 8px; }
.metric { display: grid; gap: 6px; }
.metric strong { font-size: 30px; line-height: 1.1; }
.metric span { color: var(--mall-muted); font-size: 13px; }
.section { margin-top: 16px; }
.muted { color: var(--mall-muted); font-size: 13px; }
.links { margin: 6px 0; padding-left: 18px; font-size: 12px; }
.links a { color: #2563eb; }
.brief-feedback { display: flex; align-items: center; gap: 8px; margin-top: 8px; font-size: 13px; }
h3 { margin: 0 0 12px; font-size: 16px; }
.comments { list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; }
.comments li { border: 1px solid var(--mall-border); border-radius: 8px; padding: 10px; font-size: 14px; }
.comments span { display: block; color: var(--mall-muted); font-size: 12px; margin-bottom: 4px; }
</style>
