<script setup lang="ts">
interface Address { id: number; receiver: string; phone: string; province: string; city: string; district: string; detail: string; is_default: number }
const { request, token } = useApi()
const form = reactive({ receiver: '', phone: '', province: '', city: '', district: '', detail: '', is_default: true })
const message = ref('')
const { data, refresh } = await useAsyncData('addresses', async () => {
  if (!token.value) return { addresses: [] as Address[] }
  return await request<{ addresses: Address[] }>('/api/v1/addresses')
})
async function create(): Promise<void> {
  message.value = ''
  try {
    await request('/api/v1/addresses', { method: 'POST', body: form })
    await refresh()
    Object.assign(form, { receiver: '', phone: '', province: '', city: '', district: '', detail: '', is_default: true })
  } catch (err) { message.value = err instanceof Error ? err.message : '保存失败' }
}
</script>

<template>
  <div>
    <h1>收货地址</h1>
    <div v-if="!token" class="panel"><van-empty description="请先登录" /><NuxtLink to="/login"><van-button type="primary" block>去登录</van-button></NuxtLink></div>
    <template v-else>
      <div class="panel">
        <van-cell v-for="address in data?.addresses ?? []" :key="address.id" :title="`${address.receiver} ${address.phone}`" :label="`${address.province}${address.city}${address.district}${address.detail}`">
          <template #right-icon><van-tag v-if="address.is_default" type="primary">默认</van-tag></template>
        </van-cell>
        <van-empty v-if="!data?.addresses?.length" description="还没有地址" />
      </div>
      <div class="panel form-panel">
        <h3>新增地址</h3>
        <van-form @submit="create">
          <van-field v-model="form.receiver" label="收货人" required />
          <van-field v-model="form.phone" label="手机号" required />
          <van-field v-model="form.province" label="省" />
          <van-field v-model="form.city" label="市" />
          <van-field v-model="form.district" label="区" />
          <van-field v-model="form.detail" label="详细地址" required />
          <van-button type="primary" block native-type="submit" style="margin-top:12px">保存地址</van-button>
        </van-form>
        <p v-if="message" class="muted">{{ message }}</p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.panel { background:#fff; border:1px solid #e5e7eb; border-radius:14px; padding:12px; margin-bottom:14px; }
h3 { margin:6px 0 12px; font-size:16px; }
</style>
