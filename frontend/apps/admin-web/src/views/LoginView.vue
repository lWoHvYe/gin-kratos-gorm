<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@mall/api'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const form = reactive({ username: 'admin', password: 'admin123' })
const error = ref('')

async function submit(): Promise<void> {
  error.value = ''
  try {
    await auth.login(form.username.trim(), form.password)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
    await router.replace(redirect)
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : err instanceof Error ? err.message : '登录失败'
    ElMessage.error(error.value)
  }
}
</script>

<template>
  <div class="login-shell">
    <div class="login-panel">
      <div class="brand">
        <div class="brand-mark">M</div>
        <div>
          <h1>Mall Admin</h1>
          <p>商品、促销、订单与售后运营台</p>
        </div>
      </div>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="账号">
          <el-input v-model="form.username" size="large" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" size="large" type="password" show-password autocomplete="current-password" @keyup.enter="submit" />
        </el-form-item>
        <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
        <el-button class="login-button" type="primary" size="large" :loading="auth.busy" @click="submit">
          登录
        </el-button>
      </el-form>
      <p class="hint">开发环境默认账号 admin / admin123。生产环境请移除种子账号。</p>
    </div>
  </div>
</template>

<style scoped>
.login-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  background:
    radial-gradient(circle at 20% 20%, rgba(37, 99, 235, 0.18), transparent 35%),
    radial-gradient(circle at 80% 80%, rgba(22, 163, 74, 0.12), transparent 35%),
    #f6f8fb;
  padding: 24px;
}
.login-panel {
  width: min(420px, 100%);
  background: #fff;
  border: 1px solid var(--mall-border);
  border-radius: 16px;
  padding: 30px;
  box-shadow: 0 20px 60px rgba(15, 23, 42, 0.08);
}
.brand { display: flex; gap: 14px; align-items: center; margin-bottom: 24px; }
.brand-mark {
  width: 46px; height: 46px; border-radius: 12px; display: grid; place-items: center;
  background: var(--mall-primary); color: #fff; font-weight: 800; font-size: 22px;
}
h1 { margin: 0; font-size: 22px; }
p { margin: 4px 0 0; color: var(--mall-muted); font-size: 13px; }
.login-button { width: 100%; margin-top: 16px; }
.hint { font-size: 12px; line-height: 1.6; }
</style>
