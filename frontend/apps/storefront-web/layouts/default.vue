<template>
  <div class="store-shell">
    <header class="store-header">
      <div class="container header-inner">
        <NuxtLink to="/" class="brand">Mall</NuxtLink>
        <nav class="nav">
          <NuxtLink to="/">商品</NuxtLink>
          <NuxtLink to="/seckill">秒杀</NuxtLink>
          <NuxtLink to="/orders">订单</NuxtLink>
          <NuxtLink to="/addresses">地址</NuxtLink>
          <NuxtLink to="/after-sales">售后</NuxtLink>
          <NuxtLink to="/login">登录</NuxtLink>
        </nav>
      </div>
    </header>
    <main class="container store-main"><slot /></main>
    <footer class="store-footer">Mall Store · Vue / Nuxt / Vant</footer>
    <van-tabbar route>
      <van-tabbar-item to="/" icon="home-o">首页</van-tabbar-item>
      <van-tabbar-item to="/cart" icon="cart-o">购物车</van-tabbar-item>
      <van-tabbar-item to="/orders" icon="orders-o">订单</van-tabbar-item>
      <van-tabbar-item to="/login" icon="user-o">我的</van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<style scoped>
.store-shell { min-height: 100vh; padding-bottom: 56px; }
.store-header { background: #fff; border-bottom: 1px solid #e5e7eb; position: sticky; top: 0; z-index: 10; }
.header-inner { height: 56px; display: flex; align-items: center; justify-content: space-between; }
.brand { font-size: 20px; font-weight: 800; color: var(--mall-primary); }
.nav { display: flex; gap: 16px; font-size: 14px; color: var(--mall-muted); }
.nav a.router-link-active { color: var(--mall-primary); font-weight: 600; }
.store-main { padding-top: 16px; min-height: calc(100vh - 140px); }
.store-footer { text-align: center; color: var(--mall-muted); font-size: 12px; padding: 24px 0; }
@media (max-width: 640px) { .nav { gap: 10px; font-size: 13px; } }
</style>
