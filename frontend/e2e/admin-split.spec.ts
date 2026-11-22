import { expect, test, type APIRequestContext } from '@playwright/test'

const gatewayBase = process.env.API_BASE_URL || ''
const adminBase = process.env.ADMIN_API_BASE_URL || gatewayBase
const gateway = `${gatewayBase}/api/v1`
const admin = `${adminBase}/api/admin/v1`
const skuA = Number(process.env.SPLIT_SKU_A || 10179)
const skuB = Number(process.env.SPLIT_SKU_B || 10178)

async function login(request: APIRequestContext, username: string, password: string): Promise<string> {
  for (let attempt = 0; attempt < 3; attempt++) {
    const reply = await request.post(`${gateway}/auth/login`, { data: { username, password } })
    if (reply.ok()) return (await reply.json()).access_token as string
    if (reply.status() !== 429 && reply.status() !== 401) break
    await new Promise((resolve) => setTimeout(resolve, 1000))
  }
  throw new Error(`login failed for ${username}`)
}

test('admin creates a G2 shipment for a split order', async ({ page, request }) => {
  test.setTimeout(90_000)
  const userToken = await login(request, 'demo', 'user123')
  const adminToken = await login(request, 'admin', 'admin123')
  const userHeaders = { Authorization: `Bearer ${userToken}` }
  const adminHeaders = { Authorization: `Bearer ${adminToken}` }

  for (const [sku, warehouse] of [[skuA, 1], [skuB, 2]] as const) {
    const reply = await request.post(`${admin}/inventory/adjust`, {
      headers: adminHeaders,
      data: { sku_id: sku, warehouse_id: warehouse, delta: 5, reason: 'admin split playwright' },
    })
    expect(reply.ok()).toBeTruthy()
  }

  const quoteReply = await request.post(`${gateway}/checkout/preview`, {
    headers: userHeaders,
    data: { items: [{ sku_id: skuA, quantity: 1 }, { sku_id: skuB, quantity: 1 }], address_id: 1 },
  })
  expect(quoteReply.ok()).toBeTruthy()
  const quote = await quoteReply.json()
  expect(quote.groups?.length, 'prepare stock in two different warehouses').toBe(2)

  const placeReply = await request.post(`${gateway}/checkout/place`, {
    headers: { ...userHeaders, 'Idempotency-Key': `admin-split-${Date.now()}` },
    data: {
      quote_id: quote.quote_id, address_id: 1,
      receiver_name: 'Admin Split', receiver_phone: '13800000000', receiver_address: 'Shanghai',
    },
  })
  expect(placeReply.ok()).toBeTruthy()
  const placed = await placeReply.json()
  const orderNo = placed.order.order_no as string
  const paymentNo = placed.payment.payment_no as string
  const payReply = await request.post(`${gateway}/payments/${paymentNo}/pay`, {
    headers: userHeaders,
    data: { amount_cent: quote.pay_cent, event_id: `admin-split-${paymentNo}` },
  })
  expect(payReply.ok()).toBeTruthy()

  await page.goto('/admin/login')
  const inputs = page.locator('input')
  await inputs.nth(0).fill('admin')
  await inputs.nth(1).fill('admin123')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.waitForURL((url) => url.pathname.endsWith('/admin/dashboard'))
  await page.goto(`/admin/orders/${orderNo}`)
  await expect(page.getByRole('heading', { name: '订单详情' })).toBeVisible({ timeout: 20_000 })
  await expect(page.getByText('履约分组')).toBeVisible({ timeout: 20_000 })
  const g2 = page.locator('.group').filter({ hasText: 'G2' })
  await expect(g2).toBeVisible({ timeout: 20_000 })
  await g2.getByRole('button', { name: '发货' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByLabel('物流单号').fill(`UAT-${orderNo.slice(-8)}`)
  await page.getByRole('button', { name: '创建并发货' }).click()
  await expect(page.getByText('已创建并发货')).toBeVisible({ timeout: 10_000 })

  const shipmentsReply = await request.get(`${admin}/shipments`, {
    headers: adminHeaders, params: { order_no: orderNo, page: 1, page_size: 10 },
  })
  const shipments = (await shipmentsReply.json()).shipments as Array<{ fulfillment_group_no: string; status: string }>
  const g2Shipment = shipments.find((shipment) => shipment.fulfillment_group_no === 'G2')
  expect(g2Shipment, 'G2 shipment').toBeTruthy()
  expect(g2Shipment!.status).toBe('SHIPPED')
})
