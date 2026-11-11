<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type Campaign } from '@mall/api'
import { centsFromInput, formatMoney } from '@mall/utils'

const qc = useQueryClient()
const query = useQuery({ queryKey: ['campaigns'], queryFn: () => adminApi.campaigns.list() })
const dialog = ref(false)
const editing = ref<Campaign | null>(null)
const form = reactive({
  code: '', name: '', type: 'NORMAL', status: 'DRAFT', priority: 0,
  budget: '0', per_user_limit: 0, stack_mode: 'PRIORITY',
  start: new Date(), end: new Date(Date.now() + 24 * 3600 * 1000),
})
function openCreate(): void {
  editing.value = null
  Object.assign(form, { code: '', name: '', type: 'NORMAL', status: 'DRAFT', priority: 0, budget: '0', per_user_limit: 0, stack_mode: 'PRIORITY', start: new Date(), end: new Date(Date.now() + 24 * 3600 * 1000) })
  dialog.value = true
}
function openEdit(row: Campaign): void {
  editing.value = row
  Object.assign(form, {
    code: row.code, name: row.name, type: row.type, status: row.status, priority: row.priority,
    budget: (row.budget_cent / 100).toFixed(2), per_user_limit: row.per_user_limit, stack_mode: row.stack_mode,
    start: new Date(row.start_at_unix * 1000), end: new Date(row.end_at_unix * 1000),
  })
  dialog.value = true
}
const save = useMutation({
  mutationFn: async () => {
    if (!form.code.trim() || !form.name.trim()) throw new Error('活动编码和名称必填')
    const payload = {
      code: form.code.trim(), name: form.name.trim(), type: form.type, status: form.status,
      start_at_unix: Math.floor(form.start.getTime() / 1000), end_at_unix: Math.floor(form.end.getTime() / 1000),
      priority: form.priority, budget_cent: centsFromInput(form.budget), per_user_limit: form.per_user_limit, stack_mode: form.stack_mode,
    }
    return editing.value ? adminApi.campaigns.update(editing.value.id, payload) : adminApi.campaigns.create(payload)
  },
  onSuccess: async () => { dialog.value = false; ElMessage.success('活动已保存'); await qc.invalidateQueries({ queryKey: ['campaigns'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '保存失败'),
})
const status = useMutation({
  mutationFn: ({ id, next }: { id: number; next: string }) => adminApi.campaigns.status(id, next),
  onSuccess: async () => { ElMessage.success('状态已更新'); await qc.invalidateQueries({ queryKey: ['campaigns'] }) },
  onError: (error) => ElMessage.error(error instanceof Error ? error.message : '更新失败'),
})
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">活动</h1><div class="page-subtitle">活动场负责时间、预算、优先级和叠加模式</div></div><el-button type="primary" @click="openCreate">新建活动</el-button></div>
    <div class="panel">
      <el-table :data="query.data.value?.campaigns ?? []" :loading="query.isLoading.value" empty-text="暂无活动">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="活动" min-width="220"><template #default="{ row }"><div>{{ row.name }}</div><div class="muted mono">{{ row.code }}</div></template></el-table-column>
        <el-table-column prop="type" label="类型" width="100" />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column prop="stack_mode" label="叠加" width="100" />
        <el-table-column label="预算" width="130"><template #default="{ row }">{{ formatMoney(row.budget_cent) }}</template></el-table-column>
        <el-table-column label="已消费" width="130"><template #default="{ row }">{{ formatMoney(row.consumed_cent) }}</template></el-table-column>
        <el-table-column label="时间窗" min-width="210"><template #default="{ row }"><span class="muted">{{ new Date(row.start_at_unix * 1000).toLocaleString() }} → {{ new Date(row.end_at_unix * 1000).toLocaleString() }}</span></template></el-table-column>
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button v-if="row.status === 'ACTIVE'" link type="warning" @click="status.mutate({ id: row.id, next: 'PAUSED' })">暂停</el-button>
            <el-button v-else link type="success" @click="status.mutate({ id: row.id, next: 'ACTIVE' })">启用</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dialog" :title="editing ? '编辑活动' : '新建活动'" width="640px">
      <el-form label-position="top">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="编码"><el-input v-model="form.code" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="名称"><el-input v-model="form.name" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="类型"><el-select v-model="form.type"><el-option label="普通" value="NORMAL" /><el-option label="会员" value="MEMBER" /><el-option label="秒杀（预留）" value="SECKILL" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="状态"><el-select v-model="form.status"><el-option label="草稿" value="DRAFT" /><el-option label="启用" value="ACTIVE" /><el-option label="暂停" value="PAUSED" /><el-option label="结束" value="ENDED" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="叠加模式"><el-select v-model="form.stack_mode"><el-option label="优先级" value="PRIORITY" /><el-option label="最优组合" value="BEST" /><el-option label="活动内互斥" value="EXCLUSIVE" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="开始时间"><el-date-picker v-model="form.start" type="datetime" style="width:100%" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="结束时间"><el-date-picker v-model="form.end" type="datetime" style="width:100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="优先级"><el-input-number v-model="form.priority" :min="-1000" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="预算（元，0 不限）"><el-input v-model="form.budget" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="每人限购次数"><el-input-number v-model="form.per_user_limit" :min="0" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="save.isPending.value" @click="save.mutate()">保存</el-button></template>
    </el-dialog>
  </div>
</template>
