import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'
import type {
  AfterSale,
  ApiErrorBody,
  Brand,
  Campaign,
  Category,
  LoginReply,
  Order,
  Product,
  ProductCreateInput,
  PromotionEvaluateRequest,
  PromotionEvaluation,
  PromotionReservation,
  PromotionRule,
  Refund,
  SeckillActivity,
  SeckillStock,
  Shipment,
  Sku,
  Stock,
} from './types'

interface RuntimeConfig {
  apiBase?: string
}

declare global {
  interface Window {
    __MALL_CONFIG__?: RuntimeConfig
  }
}

export class ApiError extends Error {
  readonly code: string
  readonly requestId: string
  readonly status: number

  constructor(message: string, code = 'INTERNAL', status = 0, requestId = '') {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.requestId = requestId
  }
}

function baseURL(): string {
  if (typeof window !== 'undefined' && window.__MALL_CONFIG__?.apiBase) {
    return window.__MALL_CONFIG__.apiBase.replace(/\/$/, '')
  }
  return ''
}

let accessToken = ''
let refreshToken = ''

export function setAccessToken(token: string): void {
  accessToken = token
}

export function getAccessToken(): string {
  return accessToken
}

export function setRefreshToken(token: string): void {
  refreshToken = token
}

type TokenRefreshedListener = (accessToken: string, refreshToken: string) => void
let tokenRefreshedListener: TokenRefreshedListener | null = null

// The store must persist rotated refresh tokens, otherwise a page reload would
// resurrect a revoked token.
export function onTokenRefreshed(listener: TokenRefreshedListener): void {
  tokenRefreshedListener = listener
}

export function getRefreshToken(): string {
  return refreshToken
}

// One refresh at a time: concurrent 401s share the same request so a rotating
// refresh token is not spent twice.
let refreshInFlight: Promise<string> | null = null

async function refreshAccessToken(): Promise<string> {
  if (!refreshToken) {
    throw new ApiError('session expired', 'UNAUTHENTICATED', 401)
  }
  refreshInFlight ??= axios
    .post<LoginReply>(baseURL() + '/api/v1/auth/refresh', { refresh_token: refreshToken }, { timeout: 15_000 })
    .then(({ data }) => {
      setAccessToken(data.access_token)
      setRefreshToken(data.refresh_token)
      tokenRefreshedListener?.(data.access_token, data.refresh_token)
      return data.access_token
    })
    .finally(() => {
      refreshInFlight = null
    })
  return refreshInFlight
}

function client(base: string): AxiosInstance {
  const instance = axios.create({ baseURL: baseURL() + base, timeout: 15_000 })
  instance.interceptors.request.use((config) => {
    if (accessToken) {
      config.headers.Authorization = `Bearer ${accessToken}`
    }
    return config
  })
  instance.interceptors.response.use(
    (response) => response,
    async (error) => {
      const status = Number(error?.response?.status ?? 0)
      const body = (error?.response?.data ?? {}) as ApiErrorBody
      const config = error?.config as (AxiosRequestConfig & { _retried?: boolean }) | undefined
      const isAuthCall = String(config?.url ?? '').includes('/api/v1/auth/')
      if (status === 401 && config && !config._retried && !isAuthCall) {
        try {
          const fresh = await refreshAccessToken()
          config._retried = true
          config.headers = { ...(config.headers ?? {}), Authorization: `Bearer ${fresh}` }
          return instance.request(config)
        } catch {
          setAccessToken('')
          setRefreshToken('')
        }
      }
      const message = body.message || error?.message || 'request failed'
      throw new ApiError(message, body.code || 'INTERNAL', status, body.request_id || '')
    },
  )
  return instance
}

const api = client('')
const admin = client('/api/admin/v1')

async function get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const { data } = await api.get<T>(url, config)
  return data
}
async function post<T>(url: string, body?: unknown): Promise<T> {
  const { data } = await api.post<T>(url, body)
  return data
}
async function put<T>(url: string, body?: unknown): Promise<T> {
  const { data } = await api.put<T>(url, body)
  return data
}
async function del<T>(url: string, body?: unknown): Promise<T> {
  const { data } = await api.delete<T>(url, { data: body })
  return data
}
async function adminGet<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const { data } = await admin.get<T>(url, config)
  return data
}
async function adminPost<T>(url: string, body?: unknown): Promise<T> {
  const { data } = await admin.post<T>(url, body)
  return data
}
async function adminPut<T>(url: string, body?: unknown): Promise<T> {
  const { data } = await admin.put<T>(url, body)
  return data
}
async function adminDelete<T>(url: string): Promise<T> {
  const { data } = await admin.delete<T>(url)
  return data
}

export interface PageQuery {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
}

export const authApi = {
  login: (username: string, password: string) =>
    post<LoginReply>('/api/v1/auth/login', { username, password }),
  logout: (refreshToken: string) => post<{ status: string }>('/api/v1/auth/logout', { refresh_token: refreshToken }),
}

export const adminApi = {
  categories: {
    list: () => adminGet<{ categories: Category[] }>('/categories'),
    create: (category: Partial<Category>) => adminPost<Category>('/categories', { category }),
    update: (id: number, category: Partial<Category>) => adminPut<Category>(`/categories/${id}`, { category: { ...category, id } }),
    remove: (id: number) => adminDelete<unknown>(`/categories/${id}`),
  },
  brands: {
    list: () => adminGet<{ brands: Brand[] }>('/brands'),
    create: (brand: Partial<Brand>) => adminPost<Brand>('/brands', { brand }),
    update: (id: number, brand: Partial<Brand>) => adminPut<Brand>(`/brands/${id}`, { brand: { ...brand, id } }),
    remove: (id: number) => adminDelete<unknown>(`/brands/${id}`),
  },
  products: {
    list: (query: PageQuery & { category_id?: number; brand_id?: number } = {}) =>
      adminGet<{ products: Product[]; total: number }>('/products', { params: query }),
    get: (id: number) => adminGet<Product>(`/products/${id}`),
    create: (product: ProductCreateInput) => adminPost<Product>('/products', product),
    update: (id: number, product: Partial<Product>) => adminPut<Product>(`/products/${id}`, { ...product, id }),
    remove: (id: number) => adminDelete<unknown>(`/products/${id}`),
    status: (id: number, status: string) => adminPut<unknown>(`/products/${id}/status`, { status }),
    createSku: (spuId: number, sku: Partial<Sku>) => adminPost<Sku>(`/products/${spuId}/skus`, sku),
    updateSku: (id: number, sku: Partial<Sku>) => adminPut<Sku>(`/skus/${id}`, { ...sku, id }),
    removeSku: (id: number) => adminDelete<unknown>(`/skus/${id}`),
    skuStatus: (id: number, status: string) => adminPut<Sku>(`/skus/${id}/status`, { status }),
  },
  inventory: {
    list: (query: PageQuery & { sku_id?: number; warehouse_id?: number } = {}) =>
      adminGet<{ stocks: Stock[]; total: number }>('/inventory/stocks', { params: query }),
    adjust: (skuId: number, delta: number, reason: string, warehouseId = 1) =>
      adminPost<Stock>('/inventory/adjust', { sku_id: skuId, warehouse_id: warehouseId, delta, reason }),
  },
  orders: {
    list: (query: PageQuery & { user_id?: number } = {}) =>
      adminGet<{ orders: Order[]; total: number }>('/orders', { params: query }),
    get: (orderNo: string) => adminGet<Order>(`/orders/${orderNo}`),
    cancel: (orderNo: string, reason: string) => adminPost<Order>(`/orders/${orderNo}/cancel`, { reason }),
  },
  shipments: {
    list: (query: PageQuery & { order_no?: string } = {}) =>
      adminGet<{ shipments: Shipment[]; total: number }>('/shipments', { params: query }),
    create: (orderNo: string, carrier: string, trackingNo: string, ship = true) =>
      adminPost<Shipment>('/shipments', { order_no: orderNo, carrier, tracking_no: trackingNo, ship }),
    ship: (shipmentNo: string) => adminPost<Shipment>(`/shipments/${shipmentNo}/ship`, {}),
  },
  afterSales: {
    list: (query: PageQuery & { user_id?: number } = {}) =>
      adminGet<{ after_sales: AfterSale[]; total: number }>('/after-sales', { params: query }),
    approve: (no: string, approved: boolean) => adminPost<AfterSale>(`/after-sales/${no}/approve`, { approved }),
    receive: (no: string, condition: 'SELLABLE' | 'DAMAGED') =>
      adminPost<AfterSale>(`/after-sales/${no}/receive`, { condition }),
    refund: (no: string) => adminPost<AfterSale>(`/after-sales/${no}/refund`, {}),
    refunds: (query: PageQuery & { order_no?: string } = {}) =>
      adminGet<{ refunds: Refund[]; total: number }>('/payments/refunds', { params: query }),
  },
  campaigns: {
    list: () => adminGet<{ campaigns: Campaign[] }>('/campaigns'),
    create: (campaign: Partial<Campaign>) => adminPost<Campaign>('/campaigns', campaign),
    update: (id: number, campaign: Partial<Campaign>) => adminPut<Campaign>(`/campaigns/${id}`, { ...campaign, id }),
    status: (id: number, status: string) => adminPost<unknown>(`/campaigns/${id}/status`, { status }),
  },
  seckill: {
    list: (query: PageQuery & { status?: string } = {}) =>
      adminGet<{ activities: SeckillActivity[]; total: number }>('/seckill/activities', { params: query }),
    get: (id: number) => adminGet<SeckillActivity>(`/seckill/activities/${id}`),
    create: (activity: Partial<SeckillActivity>) => adminPost<SeckillActivity>('/seckill/activities', activity),
    update: (id: number, activity: Partial<SeckillActivity>) => adminPut<SeckillActivity>(`/seckill/activities/${id}`, activity),
    publish: (id: number) => adminPost<SeckillActivity>(`/seckill/activities/${id}/publish`, {}),
    pause: (id: number) => adminPost<SeckillActivity>(`/seckill/activities/${id}/pause`, {}),
    warmup: (id: number) => adminPost<SeckillActivity>(`/seckill/activities/${id}/warmup`, {}),
    stock: (id: number) => adminGet<SeckillStock>(`/seckill/activities/${id}/stock`),
  },
  promotions: {
    list: (query: PageQuery & { campaign_id?: number } = {}) =>
      adminGet<{ promotions: PromotionRule[]; total: number }>('/promotions', { params: query }),
    create: (rule: Partial<PromotionRule>) => adminPost<PromotionRule>('/promotions', rule),
    update: (id: number, rule: Partial<PromotionRule>) => adminPut<PromotionRule>(`/promotions/${id}`, { ...rule, id }),
    status: (id: number, status: string) => adminPost<unknown>(`/promotions/${id}/status`, { status }),
    evaluate: (payload: PromotionEvaluateRequest) => adminPost<PromotionEvaluation>('/promotions/evaluate', payload),
    reservations: (query: PageQuery & { order_no?: string } = {}) =>
      adminGet<{ reservations: PromotionReservation[]; total: number }>('/promotion-reservations', { params: query }),
  },
}

export { api as http, admin }
export type { AfterSale, Brand, Campaign, Category, Order, Product, PromotionEvaluation, PromotionRule, Refund, SeckillActivity, SeckillStock, Shipment, Sku, Stock }
