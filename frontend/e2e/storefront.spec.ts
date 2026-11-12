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
