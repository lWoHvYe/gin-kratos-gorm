<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi, ApiError, type Product } from '@mall/api'
import { formatMoney } from '@mall/utils'

const qc = useQueryClient()
const page = ref(1)
const pageSize = ref(10)
const keyword = ref('')
const status = ref('')
const categoryId = ref<number | undefined>()
const query = useQuery({
  queryKey: computed(() => ['products', page.value, pageSize.value, keyword.value, status.value, categoryId.value]),
  queryFn: () => adminApi.products.list({
    page: page.value, page_size: pageSize.value, keyword: keyword.value.trim() || undefined,
    status: status.value || undefined, category_id: categoryId.value,
  }),
})
const categories = useQuery({ queryKey: ['categories'], queryFn: () => adminApi.categories.list() })
const rows = computed(() => query.data.value?.products ?? [])

const statusMutation = useMutation({
  mutationFn: ({ id, next }: { id: number; next: string }) => adminApi.products.status(id, next),
  onSuccess: async () => { ElMessage.success('状态已更新'); await qc.invalidateQueries({ queryKey: ['products'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '更新失败'),
})
const remove = useMutation({
  mutationFn: (id: number) => adminApi.products.remove(id),
  onSuccess: async () => { ElMessage.success('已删除'); await qc.invalidateQueries({ queryKey: ['products'] }) },
  onError: (error) => ElMessage.error(error instanceof Error ? error.message : '删除失败'),
})
async function toggle(row: Product): Promise<void> {
  const next = row.status === 'ON_SALE' ? 'OFF_SALE' : 'ON_SALE'
  try {
    await ElMessageBox.confirm(`确定将「${row.name}」切换为 ${next}？`, '商品状态', { type: 'warning' })
    statusMutation.mutate({ id: row.id, next })
  } catch { /* cancelled */ }
}
async function confirmRemove(row: Product): Promise<void> {
  try { await ElMessageBox.confirm(`删除商品「${row.name}」？`, '确认删除', { type: 'warning' }); remove.mutate(row.id) } catch { /* cancelled */ }
}
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">商品 / SKU</h1><div class="page-subtitle">草稿、上架、SKU 和库存展示</div></div><router-link to="/catalog/products/new"><el-button type="primary">新建商品</el-button></router-link></div>
    <div class="panel">
      <div class="toolbar">
        <el-input v-model="keyword" placeholder="搜索名称" clearable style="width: 220px" @keyup.enter="page = 1" />
        <el-select v-model="status" placeholder="全部状态" clearable style="width: 140px" @change="page = 1"><el-option label="草稿" value="DRAFT" /><el-option label="在售" value="ON_SALE" /><el-option label="下架" value="OFF_SALE" /></el-select>
        <el-select v-model="categoryId" placeholder="全部分类" clearable style="width: 160px" @change="page = 1"><el-option v-for="c in categories.data.value?.categories ?? []" :key="c.id" :label="c.name" :value="c.id" /></el-select>
        <el-button @click="page = 1">查询</el-button>
      </div>
      <el-table :data="rows" :loading="query.isLoading.value" empty-text="暂无商品">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="商品" min-width="220"><template #default="{ row }"><div>{{ row.name }}</div><div class="muted">{{ row.subtitle }}</div></template></el-table-column>
        <el-table-column prop="status" label="状态" width="110" />
        <el-table-column label="最低价" width="110"><template #default="{ row }">{{ formatMoney(row.min_price_cent) }}</template></el-table-column>
        <el-table-column label="SKU" width="80"><template #default="{ row }">{{ row.skus?.length ?? 0 }}</template></el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <router-link :to="`/catalog/products/${row.id}`"><el-button link type="primary">编辑</el-button></router-link>
            <el-button link type="primary" @click="toggle(row)">{{ row.status === 'ON_SALE' ? '下架' : '上架' }}</el-button>
            <el-button link type="danger" @click="confirmRemove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-if="(query.data.value?.total ?? 0) > pageSize" class="pager" layout="prev, pager, next, total" :total="query.data.value?.total ?? 0" :page-size="pageSize" :current-page="page" @current-change="(p:number) => page = p" />
    </div>
  </div>
</template>

<style scoped>.pager { margin-top: 16px; justify-content: flex-end; }</style>
