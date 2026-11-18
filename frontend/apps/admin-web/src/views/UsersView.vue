<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi, ApiError, type AdminUser } from '@mall/api'

const users = ref<AdminUser[]>([])
const total = ref(0)
const keyword = ref('')
const loading = ref(false)
const error = ref('')

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const reply = await adminApi.users.list({ page: 1, page_size: 100, keyword: keyword.value || undefined })
    users.value = reply.users
    total.value = reply.total
  } catch (err) {
    error.value = err instanceof ApiError || err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function toggle(user: AdminUser): Promise<void> {
  const next = user.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE'
  try {
    await ElMessageBox.confirm(
      next === 'DISABLED' ? `禁用 ${user.username}？其刷新令牌会被撤销。` : `启用 ${user.username}？`,
      '用户状态', { type: 'warning' },
    )
  } catch {
    return
  }
  try {
    await adminApi.users.status(user.id, next)
    ElMessage.success(next === 'DISABLED' ? '已禁用' : '已启用')
    await load()
  } catch (err) {
    ElMessage.error(err instanceof ApiError || err instanceof Error ? err.message : '操作失败')
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div><h1 class="page-title">用户</h1><div class="page-subtitle">共 {{ total }} 个账号；禁用会撤销刷新令牌</div></div>
    </div>
    <div class="panel">
      <div class="toolbar">
        <el-input v-model="keyword" placeholder="用户名 / 昵称 / 手机号" clearable style="max-width:320px" @keyup.enter="load" />
        <el-button type="primary" :loading="loading" @click="load">查询</el-button>
      </div>
      <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
      <el-table v-loading="loading" :data="users" empty-text="没有用户">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="username" label="用户名" min-width="140" />
        <el-table-column prop="nickname" label="昵称" min-width="140" />
        <el-table-column prop="phone" label="手机号" min-width="140" />
        <el-table-column prop="status" label="状态" width="110" />
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button size="small" :type="row.status === 'ACTIVE' ? 'danger' : 'success'" @click="toggle(row)">
              {{ row.status === 'ACTIVE' ? '禁用' : '启用' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </div>
</template>

<style scoped>
.toolbar { display: flex; gap: 10px; margin-bottom: 16px; }
</style>
