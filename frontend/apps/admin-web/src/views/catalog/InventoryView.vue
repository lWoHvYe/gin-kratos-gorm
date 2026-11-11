<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type Stock } from '@mall/api'

const qc = useQueryClient()
const skuId = ref<number | undefined>()
const page = ref(1)
const query = useQuery({
  queryKey: ['stocks', page, skuId],
  queryFn: () => adminApi.inventory.list({ page: page.value, page_size: 20, sku_id: skuId.value, warehouse_id: 1 }),
})
const dialog = ref(false)
const target = ref<Stock | null>(null)
const form = reactive({ delta: 0, reason: 'manual adjustment' })
function open(row: Stock): void { target.value = row; form.delta = 0; form.reason = 'manual adjustment'; dialog.value = true }
const adjust = useMutation({
  mutationFn: () => adminApi.inventory.adjust(target.value!.sku_id, form.delta, form.reason, target.value!.warehouse_id),
  onSuccess: async () => { dialog.value = false; ElMessage.success('库存已调整'); await qc.invalidateQueries({ queryKey: ['stocks'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '调整失败'),
})
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">库存</h1><div class="page-subtitle">inventory 是库存事实源；商品页库存是异步投影</div></div></div>
    <div class="panel">
      <div class="toolbar">
        <el-input-number v-model="skuId" :min="0" placeholder="SKU ID" controls-position="right" />
        <el-button @click="page = 1">查询</el-button>
      </div>
      <el-table :data="query.data.value?.stocks ?? []" :loading="query.isLoading.value" empty-text="暂无库存记录">
        <el-table-column prop="sku_id" label="SKU ID" width="100" />
        <el-table-column prop="warehouse_id" label="仓库" width="80" />
        <el-table-column prop="on_hand" label="实物库存" width="120" />
        <el-table-column prop="reserved" label="已预留" width="100" />
        <el-table-column label="可用" width="100"><template #default="{ row }">{{ row.available ?? Math.max(row.on_hand - row.reserved, 0) }}</template></el-table-column>
        <el-table-column label="操作" width="120"><template #default="{ row }"><el-button link type="primary" @click="open(row)">调整</el-button></template></el-table-column>
      </el-table>
      <el-pagination v-if="(query.data.value?.total ?? 0) > 20" class="pager" layout="prev, pager, next, total" :total="query.data.value?.total ?? 0" :page-size="20" :current-page="page" @current-change="(p:number) => page = p" />
    </div>
    <el-dialog v-model="dialog" title="调整库存" width="480px">
      <el-form label-position="top">
        <el-form-item label="SKU"><el-input :model-value="target?.sku_id" disabled /></el-form-item>
        <el-form-item label="变动数量（正数入库，负数出库）"><el-input-number v-model="form.delta" /></el-form-item>
        <el-form-item label="原因"><el-input v-model="form.reason" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="adjust.isPending.value" @click="adjust.mutate()">确定</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>.pager { margin-top: 16px; justify-content: flex-end; }</style>
