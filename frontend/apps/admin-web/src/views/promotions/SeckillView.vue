<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type SeckillActivity } from '@mall/api'
import { centsFromInput, formatMoney } from '@mall/utils'

const qc = useQueryClient()
const page = ref(1)
const status = ref('')
const query = useQuery({
  queryKey: computed(() => ['seckill-activities', page.value, status.value]),
  queryFn: () => adminApi.seckill.list({ page: page.value, page_size: 20, status: status.value || undefined }),
})
const campaigns = useQuery({ queryKey: ['campaigns'], queryFn: () => adminApi.campaigns.list() })
const dialog = ref(false)
const editing = ref<SeckillActivity | null>(null)
const form = reactive({
  campaign_id: 0, sku_id: 0, warehouse_id: 1, name: '', price: '0.01', stock_total: 100, per_user_limit: 1,
  start: new Date(), end: new Date(Date.now() + 24 * 3600 * 1000),
})
function openCreate(): void {
  editing.value = null
  Object.assign(form, {
    campaign_id: campaigns.data.value?.campaigns?.[0]?.id ?? 0, sku_id: 0, warehouse_id: 1, name: '',
    price: '0.01', stock_total: 100, per_user_limit: 1,
    start: new Date(), end: new Date(Date.now() + 24 * 3600 * 1000),
  })
  dialog.value = true
}
function openEdit(row: SeckillActivity): void {
  editing.value = row
  Object.assign(form, {
    campaign_id: row.campaign_id, sku_id: row.sku_id, warehouse_id: row.warehouse_id, name: row.name,
    price: (row.price_cent / 100).toFixed(2), stock_total: row.stock_total, per_user_limit: row.per_user_limit,
    start: new Date(row.start_at_unix * 1000), end: new Date(row.end_at_unix * 1000),
  })
  dialog.value = true
}
const save = useMutation({
  mutationFn: async () => {
    if (!form.campaign_id || !form.sku_id || !form.name.trim()) throw new Error('活动、SKU、名称必填')
    const payload = {
      campaign_id: form.campaign_id, sku_id: form.sku_id, warehouse_id: form.warehouse_id, name: form.name.trim(),
      price_cent: centsFromInput(form.price), stock_total: form.stock_total, per_user_limit: form.per_user_limit,
      start_at_unix: Math.floor(form.start.getTime() / 1000), end_at_unix: Math.floor(form.end.getTime() / 1000),
    }
    return editing.value ? adminApi.seckill.update(editing.value.id, { ...payload, version: editing.value.version }) : adminApi.seckill.create(payload)
  },
  onSuccess: async () => { dialog.value = false; ElMessage.success('秒杀活动已保存'); await qc.invalidateQueries({ queryKey: ['seckill-activities'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '保存失败'),
})
const command = useMutation({
  mutationFn: ({ id, action }: { id: number; action: 'publish' | 'pause' | 'warmup' }) => adminApi.seckill[action](id),
  onSuccess: async () => { ElMessage.success('操作成功'); await qc.invalidateQueries({ queryKey: ['seckill-activities'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '操作失败'),
})
const stockQuery = useQuery({
  queryKey: computed(() => ['seckill-stock', editing.value?.id]),
  queryFn: () => adminApi.seckill.stock(editing.value!.id),
  enabled: computed(() => Boolean(editing.value?.id)),
})
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">秒杀活动</h1><div class="page-subtitle">S1：活动、库存桶划拨、Redis 预热；S2 接入 Redis 准入</div></div><el-button type="primary" @click="openCreate">新建秒杀</el-button></div>
    <div class="panel">
      <div class="toolbar"><el-select v-model="status" placeholder="全部状态" clearable style="width:150px" @change="page = 1"><el-option v-for="s in ['DRAFT','READY','ACTIVE','PAUSED','ENDED','SOLD_OUT']" :key="s" :label="s" :value="s" /></el-select></div>
      <el-table :data="query.data.value?.activities ?? []" :loading="query.isLoading.value" empty-text="暂无秒杀活动">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="活动" min-width="180"><template #default="{ row }"><div>{{ row.name }}</div><div class="muted">SKU {{ row.sku_id }} · campaign {{ row.campaign_id }}</div></template></el-table-column>
        <el-table-column label="价格" width="100"><template #default="{ row }">{{ formatMoney(row.price_cent) }}</template></el-table-column>
        <el-table-column prop="stock_total" label="库存" width="80" />
        <el-table-column prop="per_user_limit" label="限购" width="70" />
        <el-table-column prop="status" label="状态" width="110" />
        <el-table-column label="时间" min-width="210"><template #default="{ row }"><span class="muted">{{ new Date(row.start_at_unix * 1000).toLocaleString() }} → {{ new Date(row.end_at_unix * 1000).toLocaleString() }}</span></template></el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'DRAFT'" link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="row.status === 'DRAFT'" link type="success" @click="command.mutate({ id: row.id, action: 'publish' })">发布</el-button>
            <el-button v-if="row.status === 'READY' || row.status === 'ACTIVE'" link type="warning" @click="command.mutate({ id: row.id, action: 'pause' })">暂停</el-button>
            <el-button v-if="row.status === 'READY' || row.status === 'ACTIVE'" link @click="command.mutate({ id: row.id, action: 'warmup' })">预热</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-if="(query.data.value?.total ?? 0) > 20" class="pager" layout="prev, pager, next, total" :total="query.data.value?.total ?? 0" :page-size="20" :current-page="page" @current-change="(p:number) => page = p" />
    </div>

    <el-dialog v-model="dialog" :title="editing ? '编辑秒杀活动' : '新建秒杀活动'" width="640px">
      <el-form label-position="top">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="活动"><el-select v-model="form.campaign_id" filterable style="width:100%"><el-option v-for="c in campaigns.data.value?.campaigns ?? []" :key="c.id" :label="c.name" :value="c.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="SKU ID"><el-input-number v-model="form.sku_id" :min="1" style="width:100%" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="名称"><el-input v-model="form.name" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="仓库 ID"><el-input-number v-model="form.warehouse_id" :min="1" style="width:100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="秒杀价（元）"><el-input v-model="form.price" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="库存"><el-input-number v-model="form.stock_total" :min="1" style="width:100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="每人限购"><el-input-number v-model="form.per_user_limit" :min="1" style="width:100%" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="开始"><el-date-picker v-model="form.start" type="datetime" style="width:100%" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="结束"><el-date-picker v-model="form.end" type="datetime" style="width:100%" /></el-form-item></el-col>
        </el-row>
        <el-alert v-if="stockQuery.data.value" type="info" :closable="false" show-icon :title="`库存桶：total=${stockQuery.data.value.total}, available=${stockQuery.data.value.available}, held=${stockQuery.data.value.held}, sold=${stockQuery.data.value.sold}`" />
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="save.isPending.value" @click="save.mutate()">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>.pager { margin-top:16px; justify-content:flex-end; }</style>
