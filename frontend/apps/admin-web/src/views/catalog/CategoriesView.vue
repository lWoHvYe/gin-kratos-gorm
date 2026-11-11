<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi, ApiError, type Category } from '@mall/api'

const qc = useQueryClient()
const query = useQuery({ queryKey: ['categories'], queryFn: () => adminApi.categories.list() })
const dialog = ref(false)
const editing = ref<Category | null>(null)
const form = reactive({ name: '', parent_id: 0, sort: 0, status: 'ENABLED' })

function openCreate(): void {
  editing.value = null
  Object.assign(form, { name: '', parent_id: 0, sort: 0, status: 'ENABLED' })
  dialog.value = true
}
function openEdit(row: Category): void {
  editing.value = row
  Object.assign(form, { name: row.name, parent_id: row.parent_id, sort: row.sort, status: row.status })
  dialog.value = true
}
const save = useMutation({
  mutationFn: async () => {
    if (!form.name.trim()) throw new Error('分类名称不能为空')
    if (editing.value) return adminApi.categories.update(editing.value.id, { ...form })
    return adminApi.categories.create({ ...form })
  },
  onSuccess: async () => {
    dialog.value = false
    ElMessage.success('保存成功')
    await qc.invalidateQueries({ queryKey: ['categories'] })
  },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '保存失败'),
})
const remove = useMutation({
  mutationFn: (id: number) => adminApi.categories.remove(id),
  onSuccess: async () => { ElMessage.success('已删除'); await qc.invalidateQueries({ queryKey: ['categories'] }) },
  onError: (error) => ElMessage.error(error instanceof Error ? error.message : '删除失败'),
})
async function confirmRemove(row: Category): Promise<void> {
  try {
    await ElMessageBox.confirm(`删除分类「${row.name}」？`, '确认删除', { type: 'warning' })
    remove.mutate(row.id)
  } catch { /* cancelled */ }
}
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div><h1 class="page-title">分类</h1><div class="page-subtitle">商品分类树和展示排序</div></div>
      <el-button type="primary" @click="openCreate">新建分类</el-button>
    </div>
    <div class="panel">
      <el-table :data="query.data.value?.categories ?? []" :loading="query.isLoading.value" empty-text="暂无分类">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column prop="parent_id" label="父分类" width="100" />
        <el-table-column prop="level" label="层级" width="80" />
        <el-table-column prop="sort" label="排序" width="80" />
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="confirmRemove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
    <el-dialog v-model="dialog" :title="editing ? '编辑分类' : '新建分类'" width="480px">
      <el-form label-position="top">
        <el-form-item label="名称"><el-input v-model="form.name" maxlength="128" /></el-form-item>
        <el-form-item label="父分类 ID"><el-input-number v-model="form.parent_id" :min="0" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" /></el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status"><el-option label="启用" value="ENABLED" /><el-option label="停用" value="DISABLED" /></el-select>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog = false">取消</el-button><el-button type="primary" :loading="save.isPending.value" @click="save.mutate()">保存</el-button></template>
    </el-dialog>
  </div>
</template>
