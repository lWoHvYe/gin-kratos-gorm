export interface ApiError { code?: string; message?: string; request_id?: string }

interface RefreshReply {
  access_token: string
  refresh_token: string
  user: { id: number; username: string; nickname: string; roles: string[] }
}

export function useApi() {
  const config = useRuntimeConfig()
  const token = useCookie<string | null>('mall_access_token', { sameSite: 'lax', secure: !import.meta.dev })
  const refreshToken = useCookie<string | null>('mall_refresh_token', { sameSite: 'lax', secure: !import.meta.dev })

  function base(): string | undefined {
    return (import.meta.server ? config.public.ssrApiBase : config.public.apiBase) || undefined
  }

  function send<T>(path: string, options: Parameters<typeof $fetch>[1] = {}): Promise<T> {
    return $fetch<T>(path, {
      baseURL: base(),
      ...options,
      headers: {
        ...(options.headers || {}),
        ...(token.value ? { Authorization: `Bearer ${token.value}` } : {}),
      },
    }) as Promise<T>
  }

  async function refresh(): Promise<boolean> {
    if (!refreshToken.value) return false
    try {
      const reply = await $fetch<RefreshReply>('/api/v1/auth/refresh', {
        baseURL: base(),
        method: 'POST',
        body: { refresh_token: refreshToken.value },
      })
      token.value = reply.access_token
      refreshToken.value = reply.refresh_token
      return true
    } catch {
      token.value = null
      refreshToken.value = null
      return false
    }
  }

  async function request<T>(path: string, options: Parameters<typeof $fetch>[1] = {}): Promise<T> {
    try {
      return await send<T>(path, options)
    } catch (err) {
      const status = (err as { statusCode?: number; response?: { status?: number } })?.response?.status
        ?? (err as { statusCode?: number })?.statusCode
      // Access tokens live 15 minutes; rotate once on 401 and replay the call.
      if (status === 401 && !path.includes('/auth/') && await refresh()) {
        return await send<T>(path, options)
      }
      throw err
    }
  }

  return { request, token, refreshToken, refresh }
}
