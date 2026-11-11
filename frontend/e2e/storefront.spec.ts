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
