<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type Shipment } from '@mall/api'
import { formatMoney } from '@mall/utils'

const route = useRoute()
const qc = useQueryClient()
const orderNo = computed(() => String(route.params.orderNo))
const order = useQuery({ queryKey: computed(() => ['order', orderNo.value]), queryFn: () => adminApi.orders.get(orderNo.value) })
const shipment = useQuery({
  queryKey: computed(() => ['shipment', orderNo.value]),
  queryFn: () => adminApi.shipments.list({ order_no: orderNo.value, page: 1, page_size: 10 }),
})
const shipDialog = ref(false)
const shipForm = reactive({ carrier: 'SF', tracking_no: '' })
const cancelReason = ref('operator cancelled')

const cancel = useMutation({
  mutationFn: () => adminApi.orders.cancel(orderNo.value, cancelReason.value),
  onSuccess: async () => { ElMessage.success('订单已取消'); await qc.invalidateQueries({ queryKey: ['order', orderNo.value] }) },
  onError: (error) => ElMessage.error(error instanceof Error ? error.message : '取消失败'),
})
const createShipment = useMutation({
  mutationFn: () => adminApi.shipments.create(orderNo.value, shipForm.carrier, shipForm.tracking_no, true),
  onSuccess: async () => { shipDialog.value = false; ElMessage.success('已创建并发货'); await qc.invalidateQueries({ queryKey: ['shipment', orderNo.value] }); await qc.invalidateQueries({ queryKey: ['order', orderNo.value] }) },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '发货失败'),
})
const currentShipment = computed<Shipment | undefined>(() => shipment.data.value?.shipments?.[0])
</script>

<template>
  <div class="page" v-loading="order.isLoading.value">
    <div class="page-header">
      <div><h1 class="page-title">订单详情</h1><div class="page-subtitle mono">{{ orderNo }}</div></div>
      <div class="toolbar" style="margin:0">
        <router-link to="/orders"><el-button>返回</el-button></router-link>
        <el-button v-if="order.data.value?.status === 'PENDING_PAYMENT'" type="danger" @click="cancel.mutate()">取消订单</el-button>
        <el-button v-if="order.data.value?.status === 'PAID'" type="primary" @click="shipDialog = true">创建发货</el-button>
      </div>
    </div>
    <el-row :gutter="16" v-if="order.data.value">
      <el-col :xs="24" :lg="16">
        <div class="panel">
          <div class="summary">
            <div><span>状态</span><strong>{{ order.data.value.status }}</strong></div>
            <div><span>应付</span><strong>{{ formatMoney(order.data.value.pay_cent) }}</strong></div>
            <div><span>已退款</span><strong>{{ formatMoney(order.data.value.refunded_cent) }}</strong></div>
            <div><span>退款状态</span><strong>{{ order.data.value.refund_status || '-' }}</strong></div>
          </div>
          <h3>商品</h3>
          <el-table :data="order.data.value.items">
            <el-table-column prop="name" label="商品" min-width="180" />
            <el-table-column prop="sku_id" label="SKU" width="90" />
            <el-table-column prop="quantity" label="数量" width="80" />
            <el-table-column label="单价" width="110"><template #default="{ row }">{{ formatMoney(row.price_cent) }}</template></el-table-column>
            <el-table-column label="优惠" width="110"><template #default="{ row }">{{ formatMoney(row.discount_cent) }}</template></el-table-column>
          </el-table>
          <div class="totals">
            <div><span>商品总额</span><b>{{ formatMoney(order.data.value.total_cent) }}</b></div>
            <div><span>优惠</span><b>-{{ formatMoney(order.data.value.discount_cent) }}</b></div>
            <div><span>运费</span><b>{{ formatMoney(order.data.value.freight_cent) }}</b></div>
            <div class="grand"><span>应付</span><b>{{ formatMoney(order.data.value.pay_cent) }}</b></div>
          </div>
          <h3>促销快照</h3>
          <pre class="pre muted">{{ order.data.value.applied_promotions_json || '无' }}</pre>
        </div>
      </el-col>
      <el-col :xs="24" :lg="8">
        <div class="panel">
          <h3>收货信息</h3>
          <p>{{ order.data.value.receiver_name }} {{ order.data.value.receiver_phone }}</p>
          <p class="muted">{{ order.data.value.receiver_address }}</p>
          <h3>发货</h3>
          <template v-if="currentShipment">
            <p>状态：{{ currentShipment.status }}</p>
            <p>{{ currentShipment.carrier }} {{ currentShipment.tracking_no }}</p>
          </template>
          <p v-else class="muted">尚未发货</p>
          <el-input v-model="cancelReason" placeholder="取消原因" />
        </div>
      </el-col>
    </el-row>
    <el-dialog v-model="shipDialog" title="创建发货" width="480px">
      <el-form label-position="top">
        <el-form-item label="承运商"><el-input v-model="shipForm.carrier" /></el-form-item>
        <el-form-item label="物流单号"><el-input v-model="shipForm.tracking_no" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="shipDialog = false">取消</el-button><el-button type="primary" :loading="createShipment.isPending.value" @click="createShipment.mutate()">创建并发货</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.summary { display:grid; grid-template-columns: repeat(4,1fr); gap:12px; margin-bottom:20px; }
.summary div { border:1px solid var(--mall-border); border-radius:8px; padding:12px; display:grid; gap:6px; }
.summary span { color:var(--mall-muted); font-size:12px; }
.summary strong { font-size:20px; }
h3 { font-size:15px; margin:20px 0 10px; }
.totals { display:grid; gap:6px; margin-top:16px; max-width:320px; margin-left:auto; }
.totals div { display:flex; justify-content:space-between; }
.totals .grand { border-top:1px solid var(--mall-border); padding-top:8px; font-size:18px; }
@media (max-width:900px){ .summary{grid-template-columns:1fr 1fr;} }
</style>
