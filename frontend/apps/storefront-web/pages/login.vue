<script setup lang="ts">
const form = reactive({ username: 'demo', password: 'user123' })
const message = ref('')
const loading = ref(false)
const { token } = useApi()
const router = useRouter()
async function submit(): Promise<void> {
  loading.value = true
  message.value = ''
  try {
    const reply = await $fetch<{ access_token: string; user: { nickname: string } }>('/api/v1/auth/login', { method: 'POST', body: form })
    token.value = reply.access_token
    await router.push('/')
  } catch (err) {
    message.value = err instanceof Error ? err.message : '登录失败'
  } finally { loading.value = false }
}
</script>

<template>
  <div class="panel">
    <h1>登录</h1>
    <van-form @submit="submit">
      <van-field v-model="form.username" label="账号" placeholder="demo" />
      <van-field v-model="form.password" label="密码" type="password" placeholder="user123" />
      <van-button type="primary" block native-type="submit" :loading="loading" style="margin-top:16px">登录</van-button>
    </van-form>
    <p v-if="message" class="muted">{{ message }}</p>
    <p class="muted small">开发环境演示账号：demo / user123</p>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:20px; max-width:520px; margin:0 auto; }
h1 { margin-top:0; }
.small { font-size:12px; }
</style>
