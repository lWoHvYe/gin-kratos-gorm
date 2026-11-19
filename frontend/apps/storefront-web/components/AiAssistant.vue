<template>
  <div class="ai-assistant">
    <button v-if="!open && available" class="ai-fab" type="button" aria-label="打开购物助手" @click="toggle">AI</button>
    <section v-else class="ai-panel" aria-label="购物助手">
      <header class="ai-head">
        <strong>购物助手</strong>
        <button type="button" class="ai-close" aria-label="关闭" @click="open = false">×</button>
      </header>
      <div ref="listEl" class="ai-messages">
        <p v-if="messages.length === 0" class="ai-hint">
          可以问我商品推荐、订单状态或退货政策。示例：“Widget 有什么”“我的订单”“退货政策”。
        </p>
        <div v-for="(m, i) in messages" :key="i" :class="['ai-msg', m.role]">
          <span>{{ m.text }}</span>
          <div v-if="m.draft" class="ai-draft">
            <strong>购物车草案</strong>
            <ul>
              <li v-for="item in m.draft.items" :key="item.sku_id">
                {{ item.name || ('SKU ' + item.sku_id) }} × {{ item.quantity }}
                <span>{{ (item.price_cent / 100).toFixed(2) }}</span>
              </li>
            </ul>
            <p>合计 {{ (m.draft.total_cent / 100).toFixed(2) }}</p>
            <button
              type="button"
              :disabled="m.draftConfirmed || m.draftSaving"
              @click="confirmDraft(m)"
            >
              {{ m.draftConfirmed ? '已加入购物车' : (m.draftSaving ? '确认中…' : '确认加入购物车') }}
            </button>
          </div>
        </div>
        <div v-if="canRate" class="ai-feedback">
          <span>有帮助吗？</span>
          <button type="button" @click="rate(1)">👍</button>
          <button type="button" @click="rate(-1)">👎</button>
          <span v-if="rated">{{ rated === 1 ? '感谢反馈' : '已记录' }}</span>
        </div>
      </div>
      <form class="ai-input" @submit.prevent="send">
        <input
          v-model="input"
          :disabled="sending"
          maxlength="2000"
          placeholder="输入问题…"
          aria-label="问题"
        />
        <button type="submit" :disabled="sending || !input.trim()">{{ sending ? '…' : '发送' }}</button>
      </form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useApi } from '~/composables/useApi'
import { parseSSEChunk, type SSEEvent } from '~/utils/sse'

interface DraftItem {
  sku_id: number
  name?: string
  price_cent: number
  quantity: number
  available: number
  stock_timestamp: number
}

interface CartDraft {
  draft_id: string
  items: DraftItem[]
  total_cent: number
  expires_at: number
}

interface Message {
  role: 'user' | 'assistant'
  text: string
  draft?: CartDraft
  draftSaving?: boolean
  draftConfirmed?: boolean
}

const { token } = useApi()
const open = ref(false)
// The server is the capability source of truth: AI_ENABLED hides the entry
// instead of leaving a dead button in the storefront.
const available = ref(false)

onMounted(async () => {
  try {
    const response = await fetch('/api/v1/ai/status')
    if (response.ok) {
      const body = await response.json() as { enabled?: boolean }
      available.value = body.enabled === true
    }
  } catch {
    // Unknown capability: keep the entry hidden rather than rendering a dead
    // button when the status endpoint or the service is unavailable.
    available.value = false
  }
})
const input = ref('')
const sending = ref(false)
const messages = ref<Message[]>([])
const rated = ref(0)
const listEl = ref<HTMLElement | null>(null)

const canRate = computed(() => {
  if (rated.value || sending.value || messages.value.length === 0) return false
  return messages.value[messages.value.length - 1]?.role === 'assistant'
    && Boolean(messages.value[messages.value.length - 1]?.text.trim())
})

async function rate(rating: 1 | -1): Promise<void> {
  if (!token.value) return
  rated.value = rating
  try {
    await fetch('/api/v1/ai/feedback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token.value}` },
      body: JSON.stringify({ session_id: sessionID(), rating }),
    })
  } catch {
    // Feedback is best-effort; never block the conversation on it.
  }
}

function toggle() {
  open.value = !open.value
  if (open.value) scrollToEnd()
}

function sessionID(): string {
  if (typeof sessionStorage === 'undefined') return 'default'
  let id = sessionStorage.getItem('mall_ai_session')
  if (!id) {
    id = crypto.randomUUID()
    sessionStorage.setItem('mall_ai_session', id)
  }
  return id
}

async function scrollToEnd() {
  await nextTick()
  if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight
}

function applyEvent(index: number, ev: SSEEvent) {
  const message = messages.value[index]
  if (!message) return
  if (ev.type === 'delta' && ev.delta) message.text += ev.delta
  if (ev.type === 'draft' && ev.draft_json) {
    try {
      message.draft = JSON.parse(ev.draft_json) as CartDraft
    } catch {
      // Ignore malformed drafts; the assistant text remains the fallback.
    }
  }
  if (ev.type === 'error') message.text = ev.error || '助手暂时不可用，请稍后再试。'
}

async function confirmDraft(message: Message): Promise<void> {
  if (!message.draft || !token.value || message.draftSaving || message.draftConfirmed) return
  message.draftSaving = true
  try {
    const response = await fetch('/api/v1/ai/cart-drafts/confirm', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token.value}` },
      body: JSON.stringify({ draft_id: message.draft.draft_id }),
    })
    const body = await response.json().catch(() => null) as { message?: string } | null
    if (!response.ok) {
      message.text += `\n${body?.message || '草案已失效，请重新生成。'}`
      return
    }
    message.draftConfirmed = true
    message.text += '\n已确认加入购物车。'
  } catch {
    message.text += '\n确认失败，请稍后重试。'
  } finally {
    message.draftSaving = false
  }
}

async function send() {
  const text = input.value.trim()
  if (!text || sending.value) return
  if (!token.value) {
    messages.value.push({ role: 'assistant', text: '请先登录后再使用购物助手。' })
    return
  }
  input.value = ''
  sending.value = true
  rated.value = 0
  messages.value.push({ role: 'user', text })
  const assistant: Message = { role: 'assistant', text: '' }
  messages.value.push(assistant)
  const index = messages.value.length - 1
  await scrollToEnd()

  try {
    const response = await fetch('/api/v1/ai/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
        Authorization: `Bearer ${token.value}`,
      },
      body: JSON.stringify({ session_id: sessionID(), message: text }),
    })
    if (!response.ok) {
      const body = await response.json().catch(() => null) as { message?: string } | null
      assistant.text = response.status === 429
        ? '提问有点频繁，请稍后再试。'
        : body?.message || '助手暂时不可用，请稍后再试。'
      return
    }
    const reader = response.body?.getReader()
    if (!reader) {
      assistant.text = '助手暂时不可用，请稍后再试。'
      return
    }
    const decoder = new TextDecoder()
    let buffer = ''
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const parsed = parseSSEChunk(buffer)
      buffer = parsed.rest
      for (const ev of parsed.events) {
        applyEvent(index, ev)
        await scrollToEnd()
        if (ev.type === 'done' || ev.type === 'error') return
      }
    }
  } catch {
    assistant.text = '网络异常，助手暂时不可用。'
  } finally {
    sending.value = false
  }
}
</script>

<style scoped>
.ai-assistant { position: fixed; right: 16px; bottom: 72px; z-index: 30; }
.ai-fab {
  width: 48px; height: 48px; border-radius: 50%; border: 0; cursor: pointer;
  background: var(--mall-primary, #2563eb); color: #fff; font-weight: 700;
  box-shadow: 0 6px 18px rgba(37, 99, 235, 0.35);
}
.ai-panel {
  width: min(360px, calc(100vw - 32px)); height: 460px; max-height: 70vh;
  display: flex; flex-direction: column; background: #fff; border-radius: 14px;
  box-shadow: 0 12px 40px rgba(15, 23, 42, 0.22); overflow: hidden;
}
.ai-head { display: flex; justify-content: space-between; align-items: center; padding: 12px 14px; border-bottom: 1px solid #e5e7eb; }
.ai-close { border: 0; background: transparent; font-size: 20px; cursor: pointer; color: #6b7280; }
.ai-messages { flex: 1; overflow-y: auto; padding: 12px; background: #f8fafc; }
.ai-hint { color: #6b7280; font-size: 13px; line-height: 1.5; }
.ai-msg { max-width: 85%; margin: 8px 0; padding: 8px 10px; border-radius: 10px; font-size: 14px; line-height: 1.5; white-space: pre-wrap; }
.ai-msg.user { margin-left: auto; background: var(--mall-primary, #2563eb); color: #fff; }
.ai-msg.assistant { margin-right: auto; background: #fff; border: 1px solid #e5e7eb; }
.ai-input { display: flex; gap: 8px; padding: 10px; border-top: 1px solid #e5e7eb; }
.ai-input input { flex: 1; min-width: 0; padding: 8px 10px; border: 1px solid #d1d5db; border-radius: 8px; }
.ai-input button { padding: 8px 14px; border: 0; border-radius: 8px; background: var(--mall-primary, #2563eb); color: #fff; cursor: pointer; }
.ai-input button:disabled { opacity: 0.5; cursor: default; }
.ai-draft {
  margin-top: 8px; padding: 10px; border: 1px solid #dbeafe; border-radius: 8px;
  background: #f8fbff; font-size: 13px;
}
.ai-draft ul { margin: 6px 0; padding-left: 18px; }
.ai-draft li { margin: 3px 0; }
.ai-draft button {
  border: 0; border-radius: 6px; padding: 6px 10px; cursor: pointer;
  background: var(--mall-primary, #2563eb); color: #fff;
}
.ai-draft button:disabled { opacity: 0.6; cursor: default; }
.ai-feedback { display: flex; align-items: center; gap: 6px; margin: 6px 0 0; color: #6b7280; font-size: 12px; }
.ai-feedback button { border: 1px solid #e5e7eb; background: #fff; border-radius: 6px; cursor: pointer; padding: 2px 6px; }
</style>
