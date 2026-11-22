import type { components as adminComponents } from './generated/admin'
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

type GeneratedCategory = adminComponents['schemas']['Category']
export type Category = Required<GeneratedCategory>

type GeneratedBrand = adminComponents['schemas']['Brand']
export type Brand = Required<GeneratedBrand>

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

type GeneratedStock = adminComponents['schemas']['Stock']
export type Stock = Omit<Required<GeneratedStock>, 'available'> & { available?: number }

export interface OrderItem {
  sku_id: number
  name: string
  price_cent: number
  quantity: number
  discount_cent: number
  warehouse_id?: number
  group_no?: string
}

export interface FulfillmentGroup {
  group_no: string
  warehouse_id: number
  freight_cent: number
  status: string
}

export interface OrderGroupAction {
  group_no: string
  action: 'ship' | 'receive' | 'apply_after_sale' | string
}

type GeneratedOrder = components['schemas']['Order']

export interface Order extends Required<Omit<GeneratedOrder, 'items' | 'fulfillment_groups' | 'group_actions'>> {
  items: OrderItem[]
  fulfillment_groups?: FulfillmentGroup[]
  group_actions?: OrderGroupAction[]
  quote_id: string
}

export interface Shipment {
  shipment_no: string
  order_no: string
  fulfillment_group_no?: string
  warehouse_id?: number
  status: string
  carrier: string
  tracking_no: string
  items: Array<{ sku_id: number; quantity: number }>
  created_at_unix: number
}

type GeneratedAfterSale = components['schemas']['AfterSale']

export interface AfterSale extends Required<GeneratedAfterSale> {
  items: Array<{ sku_id: number; quantity: number; warehouse_id?: number; group_no?: string }>
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

type GeneratedCampaign = adminComponents['schemas']['Campaign']
export type Campaign = Required<GeneratedCampaign>

type GeneratedPromotionRule = adminComponents['schemas']['PromotionRule']
export type PromotionRule = Required<GeneratedPromotionRule>

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
