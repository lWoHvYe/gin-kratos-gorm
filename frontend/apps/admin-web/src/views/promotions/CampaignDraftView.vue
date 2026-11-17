<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type PromotionEvaluation } from '@mall/api'
import { formatMoney } from '@mall/utils'

const form = reactive({
  name: '满 300 减 30',
  threshold: 30000,
  discount: 3000,
  samplePrice: 30000,
  days: 7,
  budget: 100000,
  perUser: 1,
})

const preview = ref<PromotionEvaluation | null>(null)
const saved = ref<{ campaignId: number; ruleId: number } | null>(null)
const confirmed = ref(false)
const loading = ref(false)
const error = ref('')

const startAt = computed(() => Math.floor(Date.now() / 1000) + 60)
const endAt = computed(() => startAt.value + form.days * 86400)

function payload() {
  const suffix = Date.now()
  return {
    campaign: {
      code: `UI-${suffix}`,
      name: form.name,
      type: 'PROMOTION',
      status: 'DRAFT',
      start_at_unix: startAt.value,
      end_at_unix: endAt.value,
      priority: 10,
      budget_cent: form.budget,
      per_user_limit: form.perUser,
      stack_mode: 'PRIORITY',
    },
    rule: {
      code: `UI-R-${suffix}`,
      name: form.name,
      type: 'THRESHOLD',
      status: 'DRAFT',
      priority: 10,
      scope_json: '{"all":true}',
      condition_expr: `subtotal_cent >= ${form.threshold}`,
      effect_json: JSON.stringify({ type: 'THRESHOLD', tiers: [{ min_cent: form.threshold, amount_cent: form.discount }] }),
      allow_with_coupon: true,
      start_at_unix: startAt.value,
      end_at_unix: endAt.value,
    },
    evaluation: {
      user_id: 1,
      items: [{ sku_id: 1001, spu_id: 1001, category_id: 101, brand_id: 200, price_cent: form.samplePrice, quantity: 1 }],
    },
  }
}

async function runPreview(): Promise<void> {
  error.value = ''
  preview.value = null
  saved.value = null
  confirmed.value = false
  loading.value = true
  try {
    const body = payload()
    preview.value = await adminApi.promotions.evaluateDraft({ campaign: body.campaign, rule: body.rule, evaluation: body.evaluation })
  } catch (err) {
    error.value = err instanceof ApiError || err instanceof Error ? err.message : '试算失败'
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (!confirmed.value) {
    error.value = '请先勾选“我已核对并确认保存”'
    return
  }
  error.value = ''
  loading.value = true
  try {
    const body = payload()
    const result = await adminApi.promotions.applyDraft({ campaign: body.campaign, rule: body.rule, confirm: true })
    saved.value = { campaignId: result.campaign.id, ruleId: result.rule.id }
    ElMessage.success('草案已保存为 DRAFT，未生效')
  } catch (err) {
    error.value = err instanceof ApiError || err instanceof Error ? err.message : '保存失败'
    ElMessage.error(error.value)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div>
        <h1 class="page-title">活动草案</h1>
        <div class="page-subtitle">先试算，再人工确认保存；AI 只生成草案，不直接写库</div>
      </div>
    </div>
    <el-row :gutter="16">
      <el-col :xs="24" :lg="9">
        <div class="panel">
          <el-form label-position="top">
            <el-form-item label="活动名称"><el-input v-model="form.name" /></el-form-item>
            <el-form-item label="门槛（分）"><el-input-number v-model="form.threshold" :min="1" :step="1000" /></el-form-item>
            <el-form-item label="优惠（分）"><el-input-number v-model="form.discount" :min="1" :step="500" /></el-form-item>
            <el-form-item label="试算订单金额（分）"><el-input-number v-model="form.samplePrice" :min="1" :step="1000" /></el-form-item>
            <el-form-item label="持续天数"><el-input-number v-model="form.days" :min="1" :max="90" /></el-form-item>
            <el-form-item label="预算（分）"><el-input-number v-model="form.budget" :min="0" :step="10000" /></el-form-item>
            <el-form-item label="每人限购"><el-input-number v-model="form.perUser" :min="0" /></el-form-item>
            <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
            <div class="actions">
              <el-button type="primary" :loading="loading" @click="runPreview">试算</el-button>
              <el-button type="success" :loading="loading" :disabled="!preview || !confirmed" @click="save">保存 DRAFT</el-button>
            </div>
            <el-checkbox v-model="confirmed" class="confirm">我已核对并确认保存（不会立即生效）</el-checkbox>
          </el-form>
        </div>
      </el-col>
      <el-col :xs="24" :lg="15">
        <div class="panel">
          <div v-if="preview" class="summary">
            <div><span>优惠</span><strong>{{ formatMoney(preview.discount_cent) }}</strong></div>
            <div><span>运费优惠</span><strong>{{ formatMoney(preview.shipping_discount_cent) }}</strong></div>
            <div><span>命中规则</span><strong>{{ preview.applied.length }}</strong></div>
            <div><span>拒绝原因</span><strong>{{ preview.rejected.length }}</strong></div>
          </div>
          <el-empty v-else description="先试算，确认优惠符合预期后再保存" />
          <el-table v-if="preview" :data="preview.applied" empty-text="没有命中促销">
            <el-table-column prop="name" label="促销" min-width="160" />
            <el-table-column prop="code" label="编码" min-width="160" />
            <el-table-column label="优惠" width="120"><template #default="{ row }">{{ formatMoney(row.amount_cent) }}</template></el-table-column>
          </el-table>
          <el-alert v-if="saved" class="tip" type="success" :closable="false" show-icon
            :title="`已保存 campaign ${saved.campaignId} / rule ${saved.ruleId}，状态 DRAFT`" />
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.actions { display: flex; gap: 10px; margin-top: 12px; }
.confirm { margin-top: 12px; }
.summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 16px; }
.summary div { border: 1px solid var(--mall-border); border-radius: 8px; padding: 12px; display: grid; gap: 6px; }
.summary span { color: var(--mall-muted); font-size: 12px; }
.summary strong { font-size: 22px; }
.tip { margin-top: 16px; }
@media (max-width: 900px) { .summary { grid-template-columns: 1fr 1fr; } }
</style>
