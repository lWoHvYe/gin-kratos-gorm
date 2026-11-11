<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi, ApiError, type Brand } from '@mall/api'

const qc = useQueryClient()
const query = useQuery({ queryKey: ['brands'], queryFn: () => adminApi.brands.list() })
const dialog = ref(false)
const editing = ref<Brand | null>(null)
const form = reactive({ name: '', logo: '', status: 'ENABLED' })

function openCreate(): void { editing.value = null; Object.assign(form, { name: '', logo: '', status: 'ENABLED' }); dialog.value = true }
function openEdit(row: Brand): void { editing.value = row; Object.assign(form, { name: row.name, logo: row.logo, status: row.status }); dialog.value = true }
const save = useMutation({
  mutationFn: () => editing.value
    ? adminApi.brands.update(editing.value.id, { ...form })
    : adminApi.brands.create({ ...form }),
  onSuccess: async () => { dialog.value = false; ElMessage.success('保存成功'); await qc.invalidateQueries({ queryKey: ['brands'] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '保存失败'),
})
const remove = useMutation({
  mutationFn: (id: number) => adminApi.brands.remove(id),
  onSuccess: async () => { ElMessage.success('已删除'); await qc.invalidateQueries({ queryKey: ['brands'] }) },
  onError: (error) => ElMessage.error(error instanceof Error ? error.message : '删除失败'),
})
async function confirmRemove(row: Brand): Promise<void> {
  try { await ElMessageBox.confirm(`删除品牌「${row.name}」？`, '确认删除', { type: 'warning' }); remove.mutate(row.id) } catch { /* cancelled */ }
}
</script>

<template>
  <div class="page">
    <div class="page-header"><div><h1 class="page-title">品牌</h1><div class="page-subtitle">商品品牌和 Logo</div></div><el-button type="primary" @click="openCreate">新建品牌</el-button></div>
    <div class="panel">
      <el-table :data="query.data.value?.brands ?? []" :loading="query.isLoading.value" empty-text="暂无品牌">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column prop="logo" label="Logo" min-width="220" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-button link type="danger" @click="confirmRemove(row)">删除</el-button></template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dialog" :title="editing ? '编辑品牌' : '新建品牌'" width="480px">
      <el-form label-position="top">
        <el-form-item label="名称"><el-input v-model="form.name" maxlength="128" /></el-form-item>
        <el-form-item label="Logo URL"><el-input v-model="form.logo" /></el-form-item>
        <el-form-item label="状态"><el-select v-model="form.status"><el-option label="启用" value="ENABLED" /><el-option label="停用" value="DISABLED" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="save.isPending.value" @click="save.mutate()">保存</el-button></template>
    </el-dialog>
  </div>
</template>
