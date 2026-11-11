<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { adminApi } from '@mall/api'

const page = ref(1)
const status = ref('')
const query = useQuery({
  queryKey: computed(() => ['shipments', page.value, status.value]),
  queryFn: () => adminApi.shipments.list({ page: page.value, page_size: 20, status: status.value || undefined }),
})
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">发货</h1><div class="page-subtitle">订单发货与物流状态</div></div></div>
    <div class="panel">
      <div class="toolbar"><el-select v-model="status" placeholder="全部状态" clearable style="width:150px" @change="page = 1"><el-option label="CREATED" value="CREATED" /><el-option label="SHIPPED" value="SHIPPED" /><el-option label="RECEIVED" value="RECEIVED" /></el-select></div>
      <el-table :data="query.data.value?.shipments ?? []" :loading="query.isLoading.value" empty-text="暂无发货记录">
        <el-table-column prop="shipment_no" label="发货单号" min-width="220" class="mono" />
        <el-table-column prop="order_no" label="订单号" min-width="220"><template #default="{ row }"><router-link :to="`/orders/${row.order_no}`" class="mono">{{ row.order_no }}</router-link></template></el-table-column>
        <el-table-column prop="status" label="状态" width="120" />
        <el-table-column prop="carrier" label="承运商" width="110" />
        <el-table-column prop="tracking_no" label="物流单号" min-width="150" />
        <el-table-column label="创建时间" width="180"><template #default="{ row }">{{ new Date(row.created_at_unix * 1000).toLocaleString() }}</template></el-table-column>
      </el-table>
      <el-pagination v-if="(query.data.value?.total ?? 0) > 20" class="pager" layout="prev, pager, next, total" :total="query.data.value?.total ?? 0" :page-size="20" :current-page="page" @current-change="(p:number) => page = p" />
    </div>
  </div>
</template>

<style scoped>.pager { margin-top:16px; justify-content:flex-end; }</style>
