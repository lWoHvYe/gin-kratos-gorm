<script setup lang="ts">
interface Sku { id: number; sku_code: string; specs_json: string; price_cent: number; stock: number; status: string }
interface Product { id: number; name: string; subtitle: string; description: string; main_image: string; min_price_cent: number; skus: Sku[] }
const route = useRoute()
const { request } = useApi()
const id = String(route.params.id)
const { data: product, pending, error } = await useAsyncData(`product-${id}`, () => request<Product>(`/api/v1/products/${id}`))
const selectedSku = ref<Sku | null>(null)
watchEffect(() => { if (product.value && !selectedSku.value) selectedSku.value = product.value.skus?.[0] ?? null })
const { token } = useApi()
const cartMessage = ref('')
async function addToCart(): Promise<void> {
  if (!token.value) { cartMessage.value = '请先登录'; return }
  if (!selectedSku.value) return
  try {
    await request('/api/v1/cart/items', { method: 'POST', body: { sku_id: selectedSku.value.id, quantity: 1 } })
    cartMessage.value = '已加入购物车'
  } catch (err) {
    cartMessage.value = err instanceof Error ? err.message : '加入购物车失败'
  }
}
</script>

<template>
  <div v-if="pending" class="state">加载中…</div>
  <van-empty v-else-if="error || !product" description="商品不存在" />
  <div v-else class="detail">
    <div class="gallery">
      <img v-if="product.main_image" :src="product.main_image" :alt="product.name" />
      <span v-else>{{ product.name.slice(0, 1) }}</span>
    </div>
    <section class="panel">
      <h1>{{ product.name }}</h1>
      <p class="muted">{{ product.subtitle }}</p>
      <div class="price">¥{{ ((selectedSku?.price_cent ?? product.min_price_cent) / 100).toFixed(2) }}</div>
      <div class="sku-list">
        <van-tag v-for="sku in product.skus" :key="sku.id" :type="selectedSku?.id === sku.id ? 'primary' : 'default'" size="large" @click="selectedSku = sku">
          {{ sku.sku_code }}
        </van-tag>
      </div>
      <p class="muted">{{ product.description }}</p>
      <van-button type="primary" block @click="addToCart">加入购物车</van-button>
      <p v-if="cartMessage" class="muted">{{ cartMessage }}</p>
    </section>
  </div>
</template>

<style scoped>
.detail { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
.gallery { min-height: 320px; background: #eef2ff; border-radius: 14px; display: grid; place-items: center; font-size: 80px; color: var(--mall-primary); overflow: hidden; }
.gallery img { width: 100%; height: 100%; object-fit: cover; }
.panel { background: #fff; border: 1px solid #e5e7eb; border-radius: 14px; padding: 20px; }
h1 { margin: 0 0 6px; font-size: 26px; }
.price { font-size: 28px; margin: 14px 0; }
.sku-list { display: flex; gap: 8px; flex-wrap: wrap; margin: 14px 0; cursor: pointer; }
.state { padding: 40px; text-align: center; color: var(--mall-muted); }
@media (max-width: 760px) { .detail { grid-template-columns: 1fr; } }
</style>
