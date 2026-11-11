export interface ApiError { code?: string; message?: string; request_id?: string }

export function useApi() {
  const config = useRuntimeConfig()
  const token = useCookie<string | null>('mall_access_token')

  async function request<T>(path: string, options: Parameters<typeof $fetch>[1] = {}): Promise<T> {
    const base = import.meta.server ? config.public.ssrApiBase : config.public.apiBase
    return (await $fetch<T>(path, {
      baseURL: base || undefined,
      ...options,
      headers: {
        ...(options.headers || {}),
        ...(token.value ? { Authorization: `Bearer ${token.value}` } : {}),
      },
    })) as T
  }

  return { request, token }
}
