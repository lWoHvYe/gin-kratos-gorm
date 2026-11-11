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

export interface Sku {
  id: number
  spu_id: number
  sku_code: string
  specs_json: string
  price_cent: number
  stock: number
  status: string
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

export interface Product {
  id: number
  category_id: number
  brand_id: number
  name: string
  subtitle: string
  description: string
  main_image: string
  status: string
  min_price_cent: number
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

export interface Order {
  order_no: string
  user_id: number
  status: string
  total_cent: number
  discount_cent: number
  promotion_discount_cent: number
  applied_promotions_json: string
  freight_cent: number
  pay_cent: number
  paid_cent: number
  refunded_cent: number
  refund_status: string
  receiver_name: string
  receiver_phone: string
  receiver_address: string
  items: OrderItem[]
  created_at_unix: number
  expire_at_unix: number
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

export interface AfterSale {
  after_sale_no: string
  order_no: string
  user_id: number
  type: string
  status: string
  amount_cent: number
  reason: string
  return_carrier: string
  return_tracking_no: string
  refund_no: string
  refunded_cent: number
  items: Array<{ sku_id: number; quantity: number }>
  created_at_unix: number
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
