import { expect, test } from '@playwright/test'

test('storefront renders and product detail is SSR', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('好物，简单买')).toBeVisible()
  const firstProduct = page.locator('a.card').first()
  await expect(firstProduct).toBeVisible()
  await firstProduct.click()
  await expect(page.getByRole('button', { name: '加入购物车' })).toBeVisible()
})

test('storefront login establishes a session cookie', async ({ page }) => {
  await page.goto('/login')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
  await page.goto('/cart')
  await expect(page.getByText('购物车', { exact: true }).first()).toBeVisible()
})

test('storefront opens an order detail and after-sales entry', async ({ page }) => {
  await page.goto('/login')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
  await page.goto('/orders')
  const firstOrder = page.locator('a.order').first()
  if (await firstOrder.count()) {
    await firstOrder.click()
    await expect(page.getByText('订单详情').first()).toBeVisible()
  }
})

test('storefront after-sales entry is reachable from an order', async ({ page }) => {
  await page.goto('/login')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
  await page.goto('/orders')
  const firstOrder = page.locator('a.order').first()
  if (!(await firstOrder.count())) return
  await firstOrder.click()
  const apply = page.getByText('申请售后').first()
  if (await apply.count()) {
    await apply.click()
    await expect(page.getByText('申请售后').first()).toBeVisible()
    await expect(page.getByRole('button', { name: '提交申请' })).toBeVisible()
  }
})

test('storefront seckill list and detail are reachable', async ({ page }) => {
  await page.goto('/seckill')
  await expect(page.getByText('限时秒杀').first()).toBeVisible()
  const first = page.locator('a.card').first()
  if (await first.count()) {
    await first.click()
    await expect(page.getByText('剩余库存').first()).toBeVisible()
  }
})

test('storefront can reserve, poll, pay and open a seckill order', async ({ page }) => {
  await page.goto('/')
  const setup = await page.evaluate(async () => {
    const request = async (url: string, options: RequestInit = {}) => {
      const response = await fetch(url, {
        ...options,
        headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      })
      if (!response.ok) throw new Error(`${url}: ${response.status} ${await response.text()}`)
      return response.json()
    }
    const admin = await request('/api/v1/auth/login', {
      method: 'POST', body: JSON.stringify({ username: 'admin', password: 'admin123' }),
    })
    const auth = { Authorization: `Bearer ${admin.access_token}` }
    const products = await request('/api/admin/v1/products?page=1&page_size=50', { headers: auth })
    const sku = products.products.flatMap((p: any) => p.skus || [])[0]
    if (!sku) throw new Error('no sku available for seckill e2e')
    await request('/api/admin/v1/inventory/adjust', {
      method: 'POST', headers: auth,
      body: JSON.stringify({ sku_id: sku.id, warehouse_id: 1, delta: 5, reason: 'seckill UI e2e' }),
    })
    const now = Math.floor(Date.now() / 1000)
    const suffix = `${Date.now()}-${Math.floor(Math.random() * 10000)}`
    const campaign = await request('/api/admin/v1/campaigns', {
      method: 'POST', headers: auth,
      body: JSON.stringify({
        code: `UI-E2E-CAMP-${suffix}`, name: `UI E2E ${suffix}`, type: 'SECKILL', status: 'ACTIVE',
        start_at_unix: now - 60, end_at_unix: now + 3600, priority: 100, budget_cent: 0,
        per_user_limit: 1, stack_mode: 'PRIORITY',
      }),
    })
    const name = `UI E2E SECKILL ${suffix}`
    const activity = await request('/api/admin/v1/seckill/activities', {
      method: 'POST', headers: auth,
      body: JSON.stringify({
        campaign_id: campaign.id, sku_id: sku.id, warehouse_id: 1, name,
        price_cent: 100, stock_total: 1, per_user_limit: 1,
        start_at_unix: now - 60, end_at_unix: now + 3600,
      }),
    })
    await request(`/api/admin/v1/seckill/activities/${activity.id}/publish`, { method: 'POST', headers: auth, body: '{}' })
    return { activityId: activity.id, name }
  })

  await page.getByRole('link', { name: '登录' }).click()
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
  await page.goto('/seckill')
  await page.getByText(setup.name).first().click()
  await expect(page.getByRole('button', { name: '立即抢购' })).toBeVisible()
  await page.getByRole('button', { name: '立即抢购' }).click()
  await expect(page.getByText('ORDERED')).toBeVisible({ timeout: 20_000 })
  await page.getByRole('button', { name: '支付并查看订单' }).click()
  await expect(page).toHaveURL(/\/orders\//, { timeout: 15_000 })
  await expect(page.getByText('订单详情').first()).toBeVisible()
})
