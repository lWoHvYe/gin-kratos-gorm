<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type PromotionEvaluation } from '@mall/api'
import { formatMoney } from '@mall/utils'

const form = reactive({
  user_id: 2,
  coupon_applied: false,
  items: '[\n  {"sku_id": 1, "spu_id": 1, "category_id": 1, "brand_id": 1, "price_cent": 39900, "quantity": 1}\n]',
})
const loading = ref(false)
const result = ref<PromotionEvaluation | null>(null)
const error = ref('')

interface SimItem { sku_id: number; spu_id?: number; category_id?: number; brand_id?: number; price_cent: number; quantity: number }

async function evaluate(): Promise<void> {
  error.value = ''
  result.value = null
  let items: SimItem[]
  try {
    items = JSON.parse(form.items) as SimItem[]
    if (!Array.isArray(items) || items.length === 0) throw new Error('至少一条商品')
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'items 不是合法 JSON'
    return
  }
  loading.value = true
  try {
    result.value = await adminApi.promotions.evaluate({
      user_id: form.user_id, items, coupon_applied: form.coupon_applied,
    })
  } catch (err) {
    error.value = err instanceof ApiError || err instanceof Error ? err.message : '评估失败'
    ElMessage.error(error.value)
  } finally {
    loading.value = false
  }
}

const totalItemDiscount = computed(() => result.value?.applied.reduce((sum, item) => sum + item.amount_cent, 0) ?? 0)
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">促销模拟器</h1><div class="page-subtitle">不创建订单、不预占预算，只验证当前规则会命中什么</div></div></div>
    <el-row :gutter="16">
      <el-col :xs="24" :lg="9">
        <div class="panel">
          <el-form label-position="top">
            <el-form-item label="用户 ID"><el-input-number v-model="form.user_id" :min="1" /></el-form-item>
            <el-form-item label="是否使用优惠券"><el-switch v-model="form.coupon_applied" /></el-form-item>
            <el-form-item label="商品 items JSON">
              <el-input v-model="form.items" type="textarea" :rows="12" class="mono" />
            </el-form-item>
            <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
            <el-button type="primary" :loading="loading" style="margin-top:12px" @click="evaluate">评估</el-button>
          </el-form>
        </div>
      </el-col>
      <el-col :xs="24" :lg="15">
        <div class="panel">
          <div class="summary" v-if="result">
            <div><span>促销优惠</span><strong>{{ formatMoney(result.discount_cent) }}</strong></div>
            <div><span>运费优惠</span><strong>{{ formatMoney(result.shipping_discount_cent) }}</strong></div>
            <div><span>版本</span><strong>{{ result.promotion_version }}</strong></div>
            <div><span>命中规则</span><strong>{{ result.applied.length }}</strong></div>
          </div>
          <el-empty v-else description="运行一次评估以查看命中结果" />
          <template v-if="result">
            <h3 class="section-title">命中的促销</h3>
            <el-table :data="result.applied" empty-text="没有命中促销">
              <el-table-column prop="name" label="促销" min-width="160" />
              <el-table-column prop="code" label="编码" min-width="150" />
              <el-table-column prop="type" label="类型" width="120" />
              <el-table-column label="优惠" width="110"><template #default="{ row }">{{ formatMoney(row.amount_cent) }}</template></el-table-column>
              <el-table-column prop="priority" label="优先级" width="90" />
            </el-table>
            <h3 class="section-title">未命中/被拒绝</h3>
            <el-table :data="result.rejected" empty-text="没有拒绝项">
              <el-table-column prop="code" label="编码" min-width="180" />
              <el-table-column prop="rule_id" label="规则 ID" width="100" />
              <el-table-column prop="reason" label="原因" min-width="180" />
            </el-table>
            <h3 class="section-title">优惠分摊</h3>
            <el-table :data="result.applied.flatMap((p) => p.allocations.map((a) => ({ promotion: p.name, ...a })))" empty-text="无分摊">
              <el-table-column prop="promotion" label="促销" min-width="160" />
              <el-table-column prop="sku_id" label="SKU" width="100" />
              <el-table-column label="分摊金额" width="120"><template #default="{ row }">{{ formatMoney(row.discount_cent) }}</template></el-table-column>
            </el-table>
            <div class="muted total-note">命中优惠合计：{{ formatMoney(totalItemDiscount) }}</div>
          </template>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 20px; }
.summary div { border: 1px solid var(--mall-border); border-radius: 8px; padding: 12px; display: grid; gap: 6px; }
.summary span { color: var(--mall-muted); font-size: 12px; }
.summary strong { font-size: 22px; }
.section-title { font-size: 15px; margin: 20px 0 10px; }
.total-note { margin-top: 12px; }
@media (max-width: 900px) { .summary { grid-template-columns: 1fr 1fr; } }
</style>
