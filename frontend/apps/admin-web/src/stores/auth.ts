import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { authApi, setAccessToken, type LoginUser } from '@mall/api'

const STORAGE_KEY = 'mall.admin.session'

interface StoredSession {
  accessToken: string
  refreshToken: string
  user: LoginUser
}

function readStored(): StoredSession | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredSession
    if (!parsed.accessToken || !parsed.user) return null
    return parsed
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const stored = readStored()
  const accessToken = ref(stored?.accessToken ?? '')
  const refreshToken = ref(stored?.refreshToken ?? '')
  const user = ref<LoginUser | null>(stored?.user ?? null)
  const busy = ref(false)
  setAccessToken(accessToken.value)

  const isAuthenticated = computed(() => Boolean(accessToken.value && user.value))
  const isAdmin = computed(() => Boolean(user.value?.roles?.includes('admin')))

  function persist(): void {
    if (!accessToken.value || !user.value) {
      sessionStorage.removeItem(STORAGE_KEY)
      return
    }
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({
      accessToken: accessToken.value,
      refreshToken: refreshToken.value,
      user: user.value,
    } satisfies StoredSession))
  }

  async function login(username: string, password: string): Promise<void> {
    busy.value = true
    try {
      const reply = await authApi.login(username, password)
      if (!reply.user.roles.includes('admin')) {
        throw new Error('当前账号没有后台权限')
      }
      accessToken.value = reply.access_token
      refreshToken.value = reply.refresh_token
      user.value = reply.user
      setAccessToken(reply.access_token)
      persist()
    } finally {
      busy.value = false
    }
  }

  async function logout(): Promise<void> {
    const token = refreshToken.value
    clear()
    if (token) {
      try {
        await authApi.logout(token)
      } catch {
        // Local logout must remain effective even if the server is unreachable.
      }
    }
  }

  function clear(): void {
    accessToken.value = ''
    refreshToken.value = ''
    user.value = null
    setAccessToken('')
    sessionStorage.removeItem(STORAGE_KEY)
  }

  return { accessToken, refreshToken, user, busy, isAuthenticated, isAdmin, login, logout, clear }
})
