<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { adminApi, type Order } from '@mall/api'
import { formatMoney } from '@mall/utils'

const page = ref(1)
const status = ref('')
const keyword = ref('')
const query = useQuery({
  queryKey: computed(() => ['orders', page.value, status.value, keyword.value]),
  queryFn: () => adminApi.orders.list({ page: page.value, page_size: 20, status: status.value || undefined }),
})
const rows = computed(() => (query.data.value?.orders ?? []).filter((o) => !keyword.value || o.order_no.includes(keyword.value) || o.receiver_name.includes(keyword.value)))
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">订单</h1><div class="page-subtitle">订单状态、履约、价格与退款概览</div></div></div>
    <div class="panel">
      <div class="toolbar">
        <el-input v-model="keyword" placeholder="订单号 / 收货人" clearable style="width:240px" />
        <el-select v-model="status" placeholder="全部状态" clearable style="width:180px" @change="page = 1">
          <el-option v-for="s in ['PENDING_PAYMENT','PAID','ALLOCATED','SHIPPED','COMPLETED','CANCELLED','REFUNDING','PARTIALLY_REFUNDED','REFUNDED','CLOSED']" :key="s" :label="s" :value="s" />
        </el-select>
      </div>
      <el-table :data="rows" :loading="query.isLoading.value" empty-text="暂无订单">
        <el-table-column prop="order_no" label="订单号" min-width="220"><template #default="{ row }"><router-link :to="`/orders/${row.order_no}`" class="mono">{{ row.order_no }}</router-link></template></el-table-column>
        <el-table-column prop="status" label="状态" width="160" />
        <el-table-column label="应付" width="110"><template #default="{ row }">{{ formatMoney(row.pay_cent) }}</template></el-table-column>
        <el-table-column label="已退款" width="110"><template #default="{ row }">{{ formatMoney(row.refunded_cent) }}</template></el-table-column>
        <el-table-column prop="receiver_name" label="收货人" width="120" />
        <el-table-column label="创建时间" width="180"><template #default="{ row }">{{ new Date(row.created_at_unix * 1000).toLocaleString() }}</template></el-table-column>
        <el-table-column label="操作" width="100"><template #default="{ row }"><router-link :to="`/orders/${row.order_no}`"><el-button link type="primary">详情</el-button></router-link></template></el-table-column>
      </el-table>
      <el-pagination v-if="(query.data.value?.total ?? 0) > 20" class="pager" layout="prev, pager, next, total" :total="query.data.value?.total ?? 0" :page-size="20" :current-page="page" @current-change="(p:number) => page = p" />
    </div>
  </div>
</template>

<style scoped>.pager { margin-top:16px; justify-content:flex-end; }</style>
