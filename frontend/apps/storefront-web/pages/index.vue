<script setup lang="ts">
interface Product {
  id: number
  name: string
  subtitle: string
  min_price_cent: number
  main_image: string
  skus: Array<{ id: number; stock: number }>
}
const { request } = useApi()
const { data, pending, error } = await useAsyncData('products', () => request<{ products: Product[]; total: number }>('/api/v1/products?page=1&page_size=12'))
const products = computed(() => data.value?.products ?? [])
</script>

<template>
  <div>
    <section class="hero">
      <h1>好物，简单买</h1>
      <p>浏览、搜索、下单，一条顺畅的购物路径。</p>
    </section>
    <van-search placeholder="搜索商品" disabled />
    <div v-if="pending" class="state">加载中…</div>
    <van-empty v-else-if="error" description="商品加载失败" />
    <div v-else class="grid">
      <NuxtLink v-for="product in products" :key="product.id" :to="`/products/${product.id}`" class="card">
        <div class="thumb">
          <img v-if="product.main_image" :src="product.main_image" :alt="product.name" loading="lazy" />
          <span v-else>{{ product.name.slice(0, 1) }}</span>
        </div>
        <div class="info">
          <div class="name">{{ product.name }}</div>
          <div class="muted subtitle">{{ product.subtitle }}</div>
          <div class="price">¥{{ (product.min_price_cent / 100).toFixed(2) }}</div>
        </div>
      </NuxtLink>
    </div>
  </div>
</template>

<style scoped>
.hero { background: linear-gradient(135deg, #2563eb, #0f172a); color: #fff; border-radius: 14px; padding: 26px 20px; margin-bottom: 14px; }
.hero h1 { margin: 0 0 6px; font-size: 28px; }
.hero p { margin: 0; opacity: .82; }
.grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin-top: 14px; }
.card { background: #fff; border: 1px solid #e5e7eb; border-radius: 12px; overflow: hidden; }
.thumb { height: 150px; background: #eef2ff; display: grid; place-items: center; font-size: 40px; color: var(--mall-primary); }
.thumb img { width: 100%; height: 100%; object-fit: cover; }
.info { padding: 10px; }
.name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.subtitle { font-size: 12px; margin: 4px 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.state { text-align: center; color: var(--mall-muted); padding: 40px; }
@media (max-width: 900px) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
