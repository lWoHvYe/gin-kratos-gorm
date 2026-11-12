<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { adminApi } from '@mall/api'
import { formatMoney } from '@mall/utils'

const page = ref(1)
const orderNo = ref('')
const campaigns = useQuery({ queryKey: ['campaigns'], queryFn: () => adminApi.campaigns.list() })
const reservations = useQuery({
  queryKey: computed(() => ['promotion-reservations', page.value, orderNo.value]),
  queryFn: () => adminApi.promotions.reservations({ page: page.value, page_size: 20, order_no: orderNo.value || undefined }),
})
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">促销预算</h1><div class="page-subtitle">活动预算、已消费和预占；预留随支付确认、取消释放</div></div></div>
    <div class="panel">
      <h3 class="section-title">活动预算</h3>
      <el-table :data="campaigns.data.value?.campaigns ?? []" :loading="campaigns.isLoading.value" empty-text="暂无活动">
        <el-table-column prop="name" label="活动" min-width="180" />
        <el-table-column prop="code" label="编码" min-width="160" class-name="mono" />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column label="预算" width="120"><template #default="{ row }">{{ formatMoney(row.budget_cent) }}</template></el-table-column>
        <el-table-column label="已消费" width="120"><template #default="{ row }">{{ formatMoney(row.consumed_cent) }}</template></el-table-column>
        <el-table-column label="已预占" width="120"><template #default="{ row }">{{ formatMoney(row.reserved_cent) }}</template></el-table-column>
        <el-table-column label="剩余" width="120"><template #default="{ row }">{{ formatMoney(Math.max(row.budget_cent - row.consumed_cent - row.reserved_cent, 0)) }}</template></el-table-column>
      </el-table>
    </div>
    <div class="panel section">
      <div class="panel-head"><h3>预占记录</h3><el-input v-model="orderNo" placeholder="按订单号筛选" clearable style="width:240px" /></div>
      <el-table :data="reservations.data.value?.reservations ?? []" :loading="reservations.isLoading.value" empty-text="暂无预占">
        <el-table-column prop="reservation_no" label="预占号" min-width="200" class-name="mono" />
        <el-table-column prop="order_no" label="订单号" min-width="200" class-name="mono" />
        <el-table-column prop="rule_id" label="规则" width="80" />
        <el-table-column prop="campaign_id" label="活动" width="80" />
        <el-table-column prop="status" label="状态" width="110" />
        <el-table-column label="金额" width="110"><template #default="{ row }">{{ formatMoney(row.amount_cent) }}</template></el-table-column>
        <el-table-column label="时间" width="180"><template #default="{ row }">{{ new Date(row.created_at_unix * 1000).toLocaleString() }}</template></el-table-column>
      </el-table>
      <el-pagination v-if="(reservations.data.value?.total ?? 0) > 20" class="pager" layout="prev, pager, next, total" :total="reservations.data.value?.total ?? 0" :page-size="20" :current-page="page" @current-change="(p:number) => page = p" />
    </div>
  </div>
</template>

<style scoped>
.section { margin-top: 16px; }
.section-title { font-size: 15px; margin: 0 0 12px; }
.panel-head { display:flex; align-items:center; justify-content:space-between; margin-bottom:12px; }
.panel-head h3 { margin:0; font-size:15px; }
.pager { margin-top:16px; justify-content:flex-end; }
:deep(.mono) { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
</style>
