# Accessing the mall frontends

The current live cluster is a single-node k3s VM at **10.211.55.29**. The mall
Gateway is exposed by the Istio Gateway `mall-gateway` / Service
`mall-gateway-istio` on port 80:

```text
C-end storefront : http://mall.local/
B-end admin      : http://mall.local/admin/
Gateway API      : http://mall.local/api/v1/
Admin API        : http://mall.local/api/admin/v1/
```

## 1. Primary route: Host header / hosts file

The Ingress HTTPRoute is host-based (`mall.local`), so the browser must resolve
that name to the VM. From the Mac/desktop that can reach the VM:

```bash
sudo sh -c 'grep -q "10.211.55.29 mall.local" /etc/hosts || echo "10.211.55.29 mall.local" >> /etc/hosts'
```

Then open:

```text
http://mall.local/
http://mall.local/admin/
```

On the VM itself use:

```bash
echo "127.0.0.1 mall.local" | sudo tee -a /etc/hosts
curl -H 'Host: mall.local' http://127.0.0.1/
```

If you do not want to edit `/etc/hosts`, verify with curl only:

```bash
curl --resolve mall.local:80:10.211.55.29 http://mall.local/
curl --resolve mall.local:80:10.211.55.29 -o /dev/null -w '%{http_code}\n' http://mall.local/admin/
```

`curl` without `--resolve` needs an explicit Host header:

```bash
curl -H 'Host: mall.local' http://10.211.55.29/
```

Check the external IP if it changed:

```bash
kubectl -n istio-system get svc mall-gateway-istio
```

## 2. Demo accounts

These are seed/dev accounts and must be removed or rotated for production:

```text
admin / admin123    # B-end role admin
demo  / user123     # C-end user
```

The B-end login is at `/admin/login`; the C-end login is at `/login`.

## 3. Direct pod access via port-forward

Useful when the host cannot resolve `mall.local`. The admin-web Nginx and the
Nuxt storefront now proxy `/api` to the in-cluster gateway/admin services, so
these work without the ingress:

```bash
# Terminal 1: C-end
kubectl -n mall port-forward svc/storefront-web 13000:3000

# Terminal 2: B-end
kubectl -n mall port-forward svc/admin-web 18082:8080
```

Open:

```text
C-end : http://127.0.0.1:13000/
B-end : http://127.0.0.1:18082/admin/
```

The direct paths use these in-cluster proxies:

```text
storefront-web /api/*     -> gateway:8080
admin-web      /api/v1/*  -> gateway:8080
admin-web      /api/admin/v1/* -> admin:8081
```

The storefront's SSR requests use `NUXT_PUBLIC_SSR_API_BASE=http://gateway:8080`;
browser requests use the same-origin `/api` route.

If you prefer the API port-forwards directly:

```bash
kubectl -n mall port-forward svc/gateway 18080:8080
kubectl -n mall port-forward svc/admin 18081:8081
```

Those only expose the APIs; the static/Nuxt frontends still need their own
port-forward unless accessed through `mall.local`.

## 4. Health and deployment checks

```bash
kubectl -n mall get pods
kubectl -n mall get httproute mall-routes -o wide
kubectl -n istio-system get svc mall-gateway-istio

curl -sS http://127.0.0.1:18080/healthz
curl -sS http://127.0.0.1:18081/healthz
curl -sS -H 'Host: mall.local' http://10.211.55.29/healthz
```

After rebuilding a frontend image:

```bash
kubectl -n mall rollout restart deployment/admin-web deployment/storefront-web
kubectl -n mall rollout status deployment/admin-web deployment/storefront-web
```

## 5. Troubleshooting

| Symptom | Check |
|---|---|
| `mall.local` does not resolve | Add the `/etc/hosts` entry, or use `curl --resolve` |
| Browser gets 404/403 from Gateway | HTTPRoute host is `mall.local`; use the correct Host/DNS |
| Admin UI loads but login fails | Access `/admin/` through the ingress or the admin-web port-forward; `/api` must be proxied |
| C-end loads but product API 404 | Use the storefront port-forward (it proxies `/api`) or the ingress host |
| 503 from API after a restart | Wait for Ambient/waypoint readiness, then retry; see `docs/ops.md` for certificate/data-plane recovery |
| Port-forward dies | It is a foreground kubectl stream; restart it after pod/gateway restarts |

The ingress is HTTP in this dev profile. For production, terminate TLS at the
Gateway/edge and add a real hostname, WAF/rate limits and non-demo credentials.
