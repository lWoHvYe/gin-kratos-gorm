<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type AfterSale } from '@mall/api'
import { formatMoney } from '@mall/utils'

const qc = useQueryClient()
const page = ref(1)
const status = ref('')
const query = useQuery({
  queryKey: computed(() => ['after-sales', page.value, status.value]),
  queryFn: () => adminApi.afterSales.list({ page: page.value, page_size: 20, status: status.value || undefined }),
})
const approve = useMutation({
  mutationFn: (row: AfterSale) => adminApi.afterSales.approve(row.after_sale_no, true),
  onSuccess: async () => { ElMessage.success('已审核通过'); await qc.invalidateQueries({ queryKey: ['after-sales'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '操作失败'),
})
const receive = useMutation({
  mutationFn: ({ no, condition }: { no: string; condition: 'SELLABLE' | 'DAMAGED' }) => adminApi.afterSales.receive(no, condition),
  onSuccess: async () => { ElMessage.success('已登记收货'); await qc.invalidateQueries({ queryKey: ['after-sales'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '操作失败'),
})
const refund = useMutation({
  mutationFn: (no: string) => adminApi.afterSales.refund(no),
  onSuccess: async () => { ElMessage.success('退款已完成'); await qc.invalidateQueries({ queryKey: ['after-sales'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '退款失败'),
})
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">售后</h1><div class="page-subtitle">审核、退货收货、退款；RETURN_REFUND 必须先收货</div></div></div>
    <div class="panel">
      <div class="toolbar"><el-select v-model="status" placeholder="全部状态" clearable style="width:160px" @change="page = 1"><el-option v-for="s in ['REQUESTED','APPROVED','REJECTED','RETURNING','RECEIVED','REFUNDING','REFUNDED']" :key="s" :label="s" :value="s" /></el-select></div>
      <el-table :data="query.data.value?.after_sales ?? []" :loading="query.isLoading.value" empty-text="暂无售后">
        <el-table-column prop="after_sale_no" label="售后单号" min-width="220" class="mono" />
        <el-table-column prop="order_no" label="订单号" min-width="220"><template #default="{ row }"><router-link :to="`/orders/${row.order_no}`" class="mono">{{ row.order_no }}</router-link></template></el-table-column>
        <el-table-column prop="type" label="类型" width="140" />
        <el-table-column prop="status" label="状态" width="130" />
        <el-table-column label="金额" width="110"><template #default="{ row }">{{ formatMoney(row.amount_cent) }}</template></el-table-column>
        <el-table-column prop="reason" label="原因" min-width="160" show-overflow-tooltip />
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'REQUESTED'" link type="primary" @click="approve.mutate(row)">通过</el-button>
            <template v-if="row.type === 'RETURN_REFUND' && row.status === 'RETURNING'">
              <el-button link type="success" @click="receive.mutate({ no: row.after_sale_no, condition: 'SELLABLE' })">收货（可售）</el-button>
              <el-button link type="warning" @click="receive.mutate({ no: row.after_sale_no, condition: 'DAMAGED' })">收货（残次）</el-button>
            </template>
            <el-button v-if="row.status === 'RECEIVED' || (row.type === 'REFUND_ONLY' && row.status === 'APPROVED')" link type="danger" @click="refund.mutate(row.after_sale_no)">退款</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-if="(query.data.value?.total ?? 0) > 20" class="pager" layout="prev, pager, next, total" :total="query.data.value?.total ?? 0" :page-size="20" :current-page="page" @current-change="(p:number) => page = p" />
    </div>
  </div>
</template>

<style scoped>.pager { margin-top:16px; justify-content:flex-end; }</style>
