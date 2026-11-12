<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type PromotionRule } from '@mall/api'
import { centsFromInput, formatMoney } from '@mall/utils'

const qc = useQueryClient()
const page = ref(1)
const campaignId = ref<number | undefined>()
const statusFilter = ref('')
const campaigns = useQuery({ queryKey: ['campaigns'], queryFn: () => adminApi.campaigns.list() })
const query = useQuery({
  queryKey: computed(() => ['promotions', page.value, campaignId.value, statusFilter.value]),
  queryFn: () => adminApi.promotions.list({ page: page.value, page_size: 20, campaign_id: campaignId.value, status: statusFilter.value }),
})
const dialog = ref(false)
const editing = ref<PromotionRule | null>(null)
const scopeType = ref('ALL')
const scopeIds = ref('')
const effectType = ref('FIXED_AMOUNT')
const effectAmount = ref('5.00')
const effectPercent = ref(10)
const effectMax = ref('0.00')
const effectShipping = ref('10.00')
const conditionSubtotal = ref('10.00')
const conditionQuantity = ref(1)
const form = reactive({
  campaign_id: 0, code: '', name: '', type: 'FIXED_AMOUNT', status: 'DRAFT', priority: 100,
  exclusive: false, stack_group: '', scope_json: '{"all":true}', condition_expr: 'subtotal_cent >= 1000',
  effect_json: '{"type":"FIXED_AMOUNT","amount_cent":500}', budget: '0', max_discount: '0',
  per_user_limit: 0, allow_with_coupon: true,
  start: new Date(), end: new Date(Date.now() + 24 * 3600 * 1000),
})
function openCreate(): void {
  editing.value = null
  Object.assign(form, {
    campaign_id: campaigns.data.value?.campaigns?.[0]?.id ?? 0, code: '', name: '', type: 'FIXED_AMOUNT', status: 'DRAFT', priority: 100,
    exclusive: false, stack_group: '', scope_json: '{"all":true}', condition_expr: 'subtotal_cent >= 1000',
    effect_json: '{"type":"FIXED_AMOUNT","amount_cent":500}', budget: '0', max_discount: '0', per_user_limit: 0,
    allow_with_coupon: true, start: new Date(), end: new Date(Date.now() + 24 * 3600 * 1000),
  })
  scopeType.value = 'ALL'; scopeIds.value = ''
  effectType.value = 'FIXED_AMOUNT'; effectAmount.value = '5.00'; effectPercent.value = 10; effectMax.value = '0.00'; effectShipping.value = '10.00'
  conditionSubtotal.value = '10.00'; conditionQuantity.value = 1
  dialog.value = true
}
function openEdit(row: PromotionRule): void {
  editing.value = row
  Object.assign(form, {
    campaign_id: row.campaign_id, code: row.code, name: row.name, type: row.type, status: row.status,
    priority: row.priority, exclusive: row.exclusive, stack_group: row.stack_group, scope_json: row.scope_json,
    condition_expr: row.condition_expr, effect_json: row.effect_json, budget: (row.budget_cent / 100).toFixed(2),
    max_discount: (row.max_discount_cent / 100).toFixed(2), per_user_limit: row.per_user_limit,
    allow_with_coupon: row.allow_with_coupon, start: new Date(row.start_at_unix * 1000), end: new Date(row.end_at_unix * 1000),
  })
  try {
    const scope = JSON.parse(row.scope_json) as { type?: string; ids?: number[]; all?: boolean }
    scopeType.value = scope.all ? 'ALL' : (scope.type || 'ALL')
    scopeIds.value = (scope.ids ?? []).join(',')
    const effect = JSON.parse(row.effect_json) as { type?: string; amount_cent?: number; percent?: number; max_discount_cent?: number; shipping_cent?: number }
    effectType.value = effect.type || 'FIXED_AMOUNT'
    effectAmount.value = ((effect.amount_cent ?? 0) / 100).toFixed(2)
    effectPercent.value = effect.percent ?? 10
    effectMax.value = ((effect.max_discount_cent ?? 0) / 100).toFixed(2)
    effectShipping.value = ((effect.shipping_cent ?? 1000) / 100).toFixed(2)
  } catch { /* keep raw JSON; builder is optional */ }
  dialog.value = true
}
function buildCondition(): void {
  const parts: string[] = []
  if (centsFromInput(conditionSubtotal.value) > 0) parts.push(`subtotal_cent >= ${centsFromInput(conditionSubtotal.value)}`)
  if (conditionQuantity.value > 0) parts.push(`quantity >= ${conditionQuantity.value}`)
  form.condition_expr = parts.length ? parts.join(' && ') : 'true'
}

function buildScope(): void {
  if (scopeType.value === 'ALL') { form.scope_json = '{"all":true}'; return }
  const ids = scopeIds.value.split(',').map((v) => Number(v.trim())).filter((v) => Number.isFinite(v) && v > 0)
  form.scope_json = JSON.stringify({ type: scopeType.value, ids })
}

function buildEffect(): void {
  if (effectType.value === 'FIXED_AMOUNT') {
    form.effect_json = JSON.stringify({ type: 'FIXED_AMOUNT', amount_cent: centsFromInput(effectAmount.value) })
  } else if (effectType.value === 'PERCENT') {
    form.effect_json = JSON.stringify({ type: 'PERCENT', percent: effectPercent.value, max_discount_cent: centsFromInput(effectMax.value) })
  } else if (effectType.value === 'FREE_SHIPPING') {
    form.effect_json = JSON.stringify({ type: 'FREE_SHIPPING', shipping_cent: centsFromInput(effectShipping.value) })
  }
}

function validateJSON(value: string, label: string): void {
  try { JSON.parse(value) } catch { throw new Error(`${label} 不是合法 JSON`) }
}
const save = useMutation({
  mutationFn: async () => {
    if (!form.campaign_id || !form.code.trim() || !form.name.trim()) throw new Error('活动、编码、名称必填')
    validateJSON(form.scope_json, 'scope_json')
    validateJSON(form.effect_json, 'effect_json')
    const payload = {
      campaign_id: form.campaign_id, code: form.code.trim(), name: form.name.trim(), type: form.type,
      status: form.status, priority: form.priority, exclusive: form.exclusive, stack_group: form.stack_group,
      scope_json: form.scope_json, condition_expr: form.condition_expr, effect_json: form.effect_json,
      budget_cent: centsFromInput(form.budget), max_discount_cent: centsFromInput(form.max_discount),
      per_user_limit: form.per_user_limit, allow_with_coupon: form.allow_with_coupon,
      start_at_unix: Math.floor(form.start.getTime() / 1000), end_at_unix: Math.floor(form.end.getTime() / 1000),
    }
    return editing.value ? adminApi.promotions.update(editing.value.id, payload) : adminApi.promotions.create(payload)
  },
  onSuccess: async () => { dialog.value = false; ElMessage.success('促销已保存'); await qc.invalidateQueries({ queryKey: ['promotions'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '保存失败'),
})
const status = useMutation({
  mutationFn: ({ id, next }: { id: number; next: string }) => adminApi.promotions.status(id, next),
  onSuccess: async () => { ElMessage.success('状态已更新'); await qc.invalidateQueries({ queryKey: ['promotions'] }) },
  onError: (error) => ElMessage.error(error instanceof Error ? error.message : '更新失败'),
})
const campaignName = (id: number) => campaigns.data.value?.campaigns?.find((c) => c.id === id)?.name ?? `#${id}`
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">促销规则</h1><div class="page-subtitle">条件 + 优惠效果；支持 expr 条件和类型化 effect JSON</div></div><div class="toolbar" style="margin:0"><router-link to="/promotions/simulator"><el-button>模拟评估</el-button></router-link><el-button type="primary" @click="openCreate">新建规则</el-button></div></div>
    <div class="panel">
      <div class="toolbar">
        <el-select v-model="campaignId" placeholder="全部活动" clearable style="width:180px" @change="page = 1"><el-option v-for="c in campaigns.data.value?.campaigns ?? []" :key="c.id" :label="c.name" :value="c.id" /></el-select>
        <el-select v-model="statusFilter" placeholder="全部状态" clearable style="width:140px" @change="page = 1"><el-option label="草稿" value="DRAFT" /><el-option label="启用" value="ACTIVE" /><el-option label="暂停" value="PAUSED" /></el-select>
      </div>
      <el-table :data="query.data.value?.promotions ?? []" :loading="query.isLoading.value" empty-text="暂无促销规则">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="规则" min-width="220"><template #default="{ row }"><div>{{ row.name }}</div><div class="muted mono">{{ row.code }} · {{ campaignName(row.campaign_id) }}</div></template></el-table-column>
        <el-table-column prop="type" label="类型" width="130" />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column prop="priority" label="优先级" width="90" />
        <el-table-column label="预算 / 已消费" width="150"><template #default="{ row }">{{ formatMoney(row.budget_cent) }}<span class="muted"> / {{ formatMoney(row.consumed_cent) }}</span></template></el-table-column>
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="row.status === 'ACTIVE'" link type="warning" @click="status.mutate({ id: row.id, next: 'PAUSED' })">暂停</el-button>
            <el-button v-else link type="success" @click="status.mutate({ id: row.id, next: 'ACTIVE' })">启用</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-if="(query.data.value?.total ?? 0) > 20" class="pager" layout="prev, pager, next, total" :total="query.data.value?.total ?? 0" :page-size="20" :current-page="page" @current-change="(p:number) => page = p" />
    </div>

    <el-dialog v-model="dialog" :title="editing ? '编辑促销规则' : '新建促销规则'" width="760px" top="4vh">
      <el-form label-position="top">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="所属活动"><el-select v-model="form.campaign_id" style="width:100%"><el-option v-for="c in campaigns.data.value?.campaigns ?? []" :key="c.id" :label="c.name" :value="c.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="名称"><el-input v-model="form.name" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="编码"><el-input v-model="form.code" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="类型"><el-select v-model="form.type"><el-option label="固定金额" value="FIXED_AMOUNT" /><el-option label="折扣率" value="PERCENT" /><el-option label="阶梯满减" value="THRESHOLD" /><el-option label="免运费" value="FREE_SHIPPING" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="状态"><el-select v-model="form.status"><el-option label="草稿" value="DRAFT" /><el-option label="启用" value="ACTIVE" /><el-option label="暂停" value="PAUSED" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="开始"><el-date-picker v-model="form.start" type="datetime" style="width:100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="结束"><el-date-picker v-model="form.end" type="datetime" style="width:100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="优先级"><el-input-number v-model="form.priority" :min="-1000" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="预算（元，0 不限）"><el-input v-model="form.budget" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="最大优惠（元，0 不限）"><el-input v-model="form.max_discount" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="每人限购"><el-input-number v-model="form.per_user_limit" :min="0" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="独占"><el-switch v-model="form.exclusive" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="可叠加券"><el-switch v-model="form.allow_with_coupon" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="叠加组"><el-input v-model="form.stack_group" placeholder="同组互斥" /></el-form-item></el-col>
          <el-col :span="24">
            <el-form-item label="可视化范围">
              <div class="builder-row">
                <el-select v-model="scopeType" style="width:130px"><el-option label="全部" value="ALL" /><el-option label="SKU" value="SKU" /><el-option label="SPU" value="SPU" /><el-option label="分类" value="CATEGORY" /><el-option label="品牌" value="BRAND" /></el-select>
                <el-input v-if="scopeType !== 'ALL'" v-model="scopeIds" placeholder="ID，逗号分隔，例如 1,2,3" />
                <el-button @click="buildScope">生成 scope JSON</el-button>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="24"><el-form-item label="适用范围 scope_json"><el-input v-model="form.scope_json" type="textarea" :rows="2" placeholder='{"all":true} 或 {"type":"SKU","ids":[1,2]}' /></el-form-item></el-col>
          <el-col :span="24">
            <el-form-item label="常用条件构建器">
              <div class="builder-row">
                <el-input v-model="conditionSubtotal" placeholder="最低金额（元）" style="width:150px" />
                <el-input-number v-model="conditionQuantity" :min="0" placeholder="最低件数" />
                <el-button @click="buildCondition">生成条件表达式</el-button>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="24"><el-form-item label="条件表达式 condition_expr"><el-input v-model="form.condition_expr" type="textarea" :rows="2" placeholder="subtotal_cent >= 1000 && quantity >= 2" /></el-form-item></el-col>
          <el-col :span="24">
            <el-form-item label="可视化优惠效果">
              <div class="builder-row">
                <el-select v-model="effectType" style="width:150px"><el-option label="固定金额" value="FIXED_AMOUNT" /><el-option label="折扣率" value="PERCENT" /><el-option label="免运费" value="FREE_SHIPPING" /></el-select>
                <el-input v-if="effectType === 'FIXED_AMOUNT'" v-model="effectAmount" placeholder="优惠金额（元）" style="width:150px" />
                <el-input v-if="effectType === 'PERCENT'" v-model.number="effectPercent" type="number" placeholder="折扣百分比" style="width:130px" />
                <el-input v-if="effectType === 'PERCENT'" v-model="effectMax" placeholder="最大优惠（元）" style="width:150px" />
                <el-input v-if="effectType === 'FREE_SHIPPING'" v-model="effectShipping" placeholder="运费（元）" style="width:130px" />
                <el-button @click="buildEffect">生成 effect JSON</el-button>
              </div>
            </el-form-item>
          </el-col>
          <el-col :span="24"><el-form-item label="优惠效果 effect_json"><el-input v-model="form.effect_json" type="textarea" :rows="3" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="save.isPending.value" @click="save.mutate()">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.pager { margin-top: 16px; justify-content: flex-end; }
.builder-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; width: 100%; }
</style>
