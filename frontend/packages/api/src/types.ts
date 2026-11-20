import type { components } from './generated/gateway'

export type ID = number | string

export interface ApiErrorBody {
  code?: string
  message?: string
  request_id?: string
}

export interface LoginUser {
  id: number
  username: string
  nickname: string
  roles: string[]
}

export interface AdminUser extends LoginUser {
  phone: string
  status: string
  created_at_unix: number
}

export interface LoginReply {
  access_token: string
  refresh_token: string
  user: LoginUser
}

export interface Category {
  id: number
  parent_id: number
  name: string
  level: number
  sort: number
  status: string
}

export interface Brand {
  id: number
  name: string
  logo: string
  status: string
}

type GeneratedSku = components['schemas']['Sku']

export interface Sku extends Required<GeneratedSku> {
  image_url?: string
}

export interface ProductImage {
  id?: number
  url: string
  sort?: number
}

export interface ProductAttribute {
  id?: number
  name: string
  value: string
  sort?: number
}

type GeneratedProduct = components['schemas']['Product']

export interface Product extends Required<Omit<GeneratedProduct, 'skus'>> {
  skus: Sku[]
  images?: ProductImage[]
  attributes?: ProductAttribute[]
}

export interface ListReply<T> {
  total: number
  products?: T[]
  orders?: T[]
  items?: T[]
  [key: string]: unknown
}

export interface Stock {
  sku_id: number
  warehouse_id: number
  on_hand: number
  reserved: number
  available?: number
}

export interface OrderItem {
  sku_id: number
  name: string
  price_cent: number
  quantity: number
  discount_cent: number
}

type GeneratedOrder = components['schemas']['Order']

export interface Order extends Required<Omit<GeneratedOrder, 'items'>> {
  items: OrderItem[]
  quote_id: string
}

export interface Shipment {
  shipment_no: string
  order_no: string
  status: string
  carrier: string
  tracking_no: string
  items: Array<{ sku_id: number; quantity: number }>
  created_at_unix: number
}

type GeneratedAfterSale = components['schemas']['AfterSale']

export interface AfterSale extends Required<GeneratedAfterSale> {
  items: Array<{ sku_id: number; quantity: number }>
}

export interface Refund {
  refund_no: string
  payment_no: string
  order_no: string
  amount_cent: number
  reason: string
  status: string
  created_at_unix: number
}

export interface Campaign {
  id: number
  code: string
  name: string
  type: string
  status: string
  start_at_unix: number
  end_at_unix: number
  priority: number
  budget_cent: number
  reserved_cent: number
  consumed_cent: number
  per_user_limit: number
  stack_mode: string
  version: number
}

export interface PromotionRule {
  id: number
  campaign_id: number
  code: string
  name: string
  type: string
  status: string
  priority: number
  exclusive: boolean
  stack_group: string
  scope_json: string
  condition_expr: string
  effect_json: string
  max_discount_cent: number
  budget_cent: number
  reserved_cent: number
  consumed_cent: number
  per_user_limit: number
  allow_with_coupon: boolean
  start_at_unix: number
  end_at_unix: number
  version: number
}

export interface AppliedPromotion {
  rule_id: number
  campaign_id: number
  code: string
  name: string
  type: string
  amount_cent: number
  shipping_cent: number
  allocations: Array<{ sku_id: number; discount_cent: number }>
  rule_version: number
  stack_group: string
  priority: number
}

export interface RejectedPromotion {
  rule_id: number
  code: string
  reason: string
}

export interface PromotionEvaluation {
  applied: AppliedPromotion[]
  rejected: RejectedPromotion[]
  discount_cent: number
  shipping_discount_cent: number
  promotion_version: number
  snapshot_json: string
}

export interface PromotionReservation {
  reservation_no: string
  quote_id: string
  order_no: string
  user_id: number
  rule_id: number
  campaign_id: number
  amount_cent: number
  status: string
  created_at_unix: number
}

export interface PromotionEvaluateRequest {
  user_id: number
  items: Array<{
    sku_id: number
    spu_id?: number
    category_id?: number
    brand_id?: number
    price_cent: number
    quantity: number
  }>
  coupon_applied?: boolean
  coupon_discount_cent?: number
}

export interface SkuInput {
  sku_code: string
  specs_json: string
  price_cent: number
  image_url?: string
}

export interface ProductCreateInput {
  category_id: number
  brand_id?: number
  name: string
  subtitle?: string
  description?: string
  main_image?: string
  images?: ProductImage[]
  attributes?: ProductAttribute[]
  skus: SkuInput[]
}

export interface SeckillActivity {
  id: number
  campaign_id: number
  sku_id: number
  warehouse_id: number
  name: string
  price_cent: number
  stock_total: number
  per_user_limit: number
  start_at_unix: number
  end_at_unix: number
  status: string
  version: number
  created_at_unix: number
  updated_at_unix: number
}

export interface SeckillStock {
  activity_id: number
  sku_id: number
  warehouse_id: number
  total: number
  available: number
  held: number
  sold: number
  status: string
  version: number
}

export interface FeedbackDay {
  day: string
  helpful: number
  not_helpful: number
}

export interface OpsBrief {
  summary: string
  total_orders: number
  paid_orders: number
  paid_cent: number
  after_sales: number
  refunded_cent: number
  low_stock: number
  generated_at: number
  exception_links?: string[]
}

export interface FeedbackSummary {
  total: number
  helpful: number
  not_helpful: number
  days: FeedbackDay[]
  not_helpful_comments: Array<{ day: string; comment: string }>
}
