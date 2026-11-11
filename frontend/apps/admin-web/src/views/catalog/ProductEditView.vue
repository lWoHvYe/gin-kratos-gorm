<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ElMessage } from 'element-plus'
import { adminApi, ApiError, type Product, type Sku } from '@mall/api'
import { formatMoney, centsFromInput } from '@mall/utils'

interface SkuDraft {
  id?: number
  sku_code: string
  specs_json: string
  price: string
  status: string
  image_url: string
  stock: number
}

const route = useRoute()
const router = useRouter()
const qc = useQueryClient()
const isNew = computed(() => route.name === 'product-new')
const productId = computed(() => Number(route.params.id))
const categories = useQuery({ queryKey: ['categories'], queryFn: () => adminApi.categories.list() })
const brands = useQuery({ queryKey: ['brands'], queryFn: () => adminApi.brands.list() })
const product = useQuery({
  queryKey: computed(() => ['product', productId.value]),
  queryFn: () => adminApi.products.get(productId.value),
  enabled: computed(() => !isNew.value && Number.isFinite(productId.value)),
})
const form = reactive({ category_id: 0, brand_id: 0, name: '', subtitle: '', description: '', main_image: '', status: 'DRAFT' })
const skus = ref<SkuDraft[]>([])
const skuDialog = ref(false)
const skuDraft = reactive<SkuDraft>({ sku_code: '', specs_json: '{}', price: '', status: 'ENABLED', image_url: '', stock: 0 })
const loaded = ref(false)

watch(() => product.data.value, (value?: Product) => {
  if (!value || loaded.value) return
  Object.assign(form, {
    category_id: value.category_id, brand_id: value.brand_id, name: value.name,
    subtitle: value.subtitle, description: value.description, main_image: value.main_image, status: value.status,
  })
  skus.value = (value.skus ?? []).map((sku) => ({
    id: sku.id, sku_code: sku.sku_code, specs_json: sku.specs_json,
    price: (sku.price_cent / 100).toFixed(2), status: sku.status, image_url: sku.image_url ?? '', stock: sku.stock,
  }))
  loaded.value = true
}, { immediate: true })

function addSkuDraft(): void {
  Object.assign(skuDraft, { sku_code: '', specs_json: '{}', price: '', status: 'ENABLED', image_url: '', stock: 0 })
  skuDialog.value = true
}
function confirmSku(): void {
  if (!skuDraft.sku_code.trim() || centsFromInput(skuDraft.price) <= 0) {
    ElMessage.error('SKU 编码和价格必填')
    return
  }
  const existing = skus.value.find((s) => s.sku_code === skuDraft.sku_code.trim())
  if (existing) {
    Object.assign(existing, { ...skuDraft, price: skuDraft.price, sku_code: skuDraft.sku_code.trim() })
  } else {
    skus.value.push({ ...skuDraft, sku_code: skuDraft.sku_code.trim() })
  }
  skuDialog.value = false
}

const save = useMutation({
  mutationFn: async () => {
    if (!form.name.trim() || !form.category_id) throw new Error('商品名称和分类必填')
    const payload = {
      category_id: form.category_id,
      brand_id: form.brand_id,
      name: form.name.trim(),
      subtitle: form.subtitle,
      description: form.description,
      main_image: form.main_image,
    }
    if (isNew.value) {
      return adminApi.products.create({
        ...payload,
        skus: skus.value.map((sku) => ({
          sku_code: sku.sku_code,
          specs_json: sku.specs_json || '{}',
          price_cent: centsFromInput(sku.price),
          image_url: sku.image_url,
        })),
      })
    }
    return adminApi.products.update(productId.value, payload)
  },
  onSuccess: async (value) => {
    ElMessage.success('商品已保存')
    await qc.invalidateQueries({ queryKey: ['products'] })
    if (isNew.value) await router.replace(`/catalog/products/${value.id}`)
    else await qc.invalidateQueries({ queryKey: ['product', productId.value] })
  },
  onError: (error) => ElMessage.error(error instanceof ApiError || error instanceof Error ? error.message : '保存失败'),
})

async function addSkuToServer(sku: SkuDraft): Promise<void> {
  if (!sku.id) return
  try {
    await adminApi.products.updateSku(sku.id, {
      sku_code: sku.sku_code, specs_json: sku.specs_json || '{}',
      price_cent: centsFromInput(sku.price), status: sku.status, image_url: sku.image_url,
    })
    ElMessage.success('SKU 已更新')
    await qc.invalidateQueries({ queryKey: ['product', productId.value] })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'SKU 更新失败')
  }
}

const statusMutation = useMutation({
  mutationFn: (next: string) => adminApi.products.status(productId.value, next),
  onSuccess: async () => { ElMessage.success('状态已更新'); await qc.invalidateQueries({ queryKey: ['product', productId.value] }) },
  onError: (error) => ElMessage.error(error instanceof Error ? error.message : '状态更新失败'),
})

function newSkuToServer(sku: SkuDraft): void {
  if (!sku.id) {
    adminApi.products.createSku(productId.value, {
      sku_code: sku.sku_code, specs_json: sku.specs_json || '{}',
      price_cent: centsFromInput(sku.price), image_url: sku.image_url,
    }).then(async () => {
      ElMessage.success('SKU 已添加')
      await qc.invalidateQueries({ queryKey: ['product', productId.value] })
    }).catch((error: unknown) => ElMessage.error(error instanceof Error ? error.message : 'SKU 添加失败'))
  } else {
    void addSkuToServer(sku)
  }
}
</script>

<template>
  <div class="page">
    <div class="page-header">
      <div><h1 class="page-title">{{ isNew ? '新建商品' : '编辑商品' }}</h1><div class="page-subtitle">商品创建后为 DRAFT，初始化库存后再上架</div></div>
      <div class="toolbar" style="margin:0"><router-link to="/catalog/products"><el-button>返回</el-button></router-link><el-button type="primary" :loading="save.isPending.value" @click="save.mutate()">保存商品</el-button></div>
    </div>
    <div class="panel" v-loading="!isNew && product.isLoading.value">
      <el-form label-position="top">
        <el-row :gutter="16">
          <el-col :xs="24" :md="12"><el-form-item label="商品名称"><el-input v-model="form.name" maxlength="255" /></el-form-item></el-col>
          <el-col :xs="24" :md="12"><el-form-item label="副标题"><el-input v-model="form.subtitle" maxlength="255" /></el-form-item></el-col>
          <el-col :xs="24" :md="12"><el-form-item label="分类"><el-select v-model="form.category_id" filterable style="width:100%"><el-option v-for="c in categories.data.value?.categories ?? []" :key="c.id" :label="c.name" :value="c.id" /></el-select></el-form-item></el-col>
          <el-col :xs="24" :md="12"><el-form-item label="品牌"><el-select v-model="form.brand_id" clearable style="width:100%"><el-option v-for="b in brands.data.value?.brands ?? []" :key="b.id" :label="b.name" :value="b.id" /></el-select></el-form-item></el-col>
          <el-col :xs="24"><el-form-item label="主图 URL"><el-input v-model="form.main_image" /></el-form-item></el-col>
          <el-col :xs="24"><el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="4" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <div v-if="!isNew" class="toolbar">
        <span>当前状态：<el-tag>{{ form.status }}</el-tag></span>
        <el-button size="small" @click="statusMutation.mutate(form.status === 'ON_SALE' ? 'OFF_SALE' : 'ON_SALE')">{{ form.status === 'ON_SALE' ? '下架' : '上架' }}</el-button>
      </div>
    </div>

    <div class="panel section">
      <div class="panel-head"><h3>SKU</h3><el-button size="small" @click="addSkuDraft">添加 SKU</el-button></div>
      <el-table :data="skus" empty-text="暂无 SKU">
        <el-table-column prop="sku_code" label="SKU 编码" min-width="160" />
        <el-table-column label="价格" width="110"><template #default="{ row }">{{ formatMoney(centsFromInput(row.price)) }}</template></el-table-column>
        <el-table-column prop="status" label="状态" width="100" />
        <el-table-column prop="stock" label="库存（投影）" width="130" />
        <el-table-column label="操作" width="160">
          <template #default="{ row }">
            <el-button v-if="!isNew" link type="primary" @click="newSkuToServer(row)">{{ row.id ? '保存' : '创建' }}</el-button>
            <el-button link type="danger" @click="skus = skus.filter((s) => s !== row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-alert class="tip" type="info" :closable="false" show-icon title="库存由 inventory 管理。SKU 创建后请到库存页调整初始库存，再上架商品。" />
    </div>

    <el-dialog v-model="skuDialog" title="添加 SKU" width="480px">
      <el-form label-position="top">
        <el-form-item label="SKU 编码"><el-input v-model="skuDraft.sku_code" /></el-form-item>
        <el-form-item label="价格（元）"><el-input v-model="skuDraft.price" placeholder="例如 99.00" /></el-form-item>
        <el-form-item label="规格 JSON"><el-input v-model="skuDraft.specs_json" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="图片 URL"><el-input v-model="skuDraft.image_url" /></el-form-item>
        <el-form-item label="状态"><el-select v-model="skuDraft.status"><el-option label="启用" value="ENABLED" /><el-option label="停用" value="DISABLED" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="skuDialog = false">取消</el-button><el-button type="primary" @click="confirmSku">确定</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.section { margin-top: 16px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.panel-head h3 { margin: 0; font-size: 16px; }
.tip { margin-top: 14px; }
</style>
