import { proxyRequest } from 'h3'

// Direct pod/port-forward access: the ingress normally routes /api before the
// storefront, but a kubectl port-forward to :3000 needs the same-origin API
// proxy. The target is runtime configuration, not a build-time constant.
export default defineEventHandler((event) => {
  const target = process.env.NUXT_API_PROXY_TARGET
  if (!target || !event.path.startsWith('/api/')) return
  return proxyRequest(event, target.replace(/\/$/, '') + event.path)
})
