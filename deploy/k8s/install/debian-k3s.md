# Debian VM + single-node k3s + Istio Ambient

Validated on the current VM:

- Debian 12, 4 vCPU, 16 GiB RAM
- k3s `v1.37.2+k3s1`
- Istio `1.31.4` (Ambient)
- k3s data and kubelet data on `/home` (the root `/` and `/var` disks are small)
- Docker kept installed and moved to `/home/docker`; k3s uses containerd

## 1. Prepare the VM

```bash
ssh-copy-id root@10.211.55.29
ssh root@10.211.55.29

apt update
apt install -y curl ca-certificates jq conntrack socat
hostnamectl set-hostname mall-k8s
nproc
free -h
df -h /
```

The VM must have at least 4 vCPU and 8 GiB RAM. This deployment used 16 GiB.

## 2. Proxy and DNS

The VM's mihomo listens on `127.0.0.1:7890`. Keep its HTTP proxy, but **do not
let its TUN DNS hijack Kubernetes CIDRs**. Add this to
`/root/.config/mihomo/config.yaml`:

```yaml
tun:
  enable: true
  stack: mixed
  dns-hijack:
    - "8.8.8.8:53"
    - "1.1.1.1:53"
  auto-route: true
  auto-redirect: true
  auto-detect-interface: true
  route-exclude-address:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
```

Then reload mihomo (or restart it). Without the exclusion, ClusterIP DNS
(`10.43.0.10`) is hijacked and CoreDNS returns fake IPs.

Configure k3s proxy environment in
`/etc/systemd/system/k3s.service.d/proxy.conf`:

```ini
[Service]
Environment="HTTP_PROXY=http://127.0.0.1:7890"
Environment="HTTPS_PROXY=http://127.0.0.1:7890"
Environment="NO_PROXY=127.0.0.1,localhost,10.211.55.0/24,10.42.0.0/16,10.43.0.0/16,.svc,.cluster.local"
Environment="http_proxy=http://127.0.0.1:7890"
Environment="https_proxy=http://127.0.0.1:7890"
Environment="no_proxy=127.0.0.1,localhost,10.211.55.0/24,10.42.0.0/16,10.43.0.0/16,.svc,.cluster.local"
```

## 3. Install k3s

Keep Traefik disabled; the chart's Gateway API Gateway provides Istio ingress.
Use `/home/k3s` because `/var` is small.

```bash
export http_proxy=http://127.0.0.1:7890 https_proxy=http://127.0.0.1:7890
export NO_PROXY=127.0.0.1,localhost,10.211.55.0/24,10.42.0.0/16,10.43.0.0/16,.svc,.cluster.local

mkdir -p /home/k3s
chmod 700 /home/k3s

curl -sfL https://get.k3s.io | INSTALL_K3S_SKIP_START=true INSTALL_K3S_EXEC="server \
  --write-kubeconfig-mode 644 \
  --tls-san 10.211.55.29 \
  --node-external-ip 10.211.55.29 \
  --disable traefik \
  --data-dir /home/k3s" sh -

mkdir -p /etc/systemd/system/k3s.service.d
# create proxy.conf as shown above
systemctl daemon-reload
systemctl enable --now k3s
```

Move the kubelet root to `/home` so ephemeral storage is not limited by the
small `/var` partition:

```yaml
# /etc/rancher/k3s/config.yaml
kubelet-arg:
  - root-dir=/home/k3s/kubelet
```

Then restart k3s and check:

```bash
systemctl restart k3s
kubectl get node -o json | jq '.items[0].status.capacity'
```

`ephemeral-storage` should now report the `/home` capacity (about 45 GiB).

## 4. Mac kubeconfig

```bash
scp root@10.211.55.29:/etc/rancher/k3s/k3s.yaml ~/.kube/mall-k3s.yaml
sed -i '' 's/127.0.0.1/10.211.55.29/' ~/.kube/mall-k3s.yaml
export KUBECONFIG=$HOME/.kube/mall-k3s.yaml
kubectl get nodes
```

If `kubectl` is not installed on the Mac, install a matching client or use
`ssh root@10.211.55.29 k3s kubectl` for bootstrap.

## 5. Install Istio Ambient

Pin `1.31.4` (or the current supported version in
`deploy/k8s/istio/version`):

```bash
istioctl install --set profile=ambient \
  --set values.cni.cniConfDir=/home/k3s/agent/etc/cni/net.d \
  --set values.cni.cniBinDir=/home/k3s/data/cni \
  --set values.pilot.env.ENABLE_INGRESS_WAYPOINT_ROUTING=true \
  --skip-confirmation
```

The k3s CNI paths are **not** `/etc/cni/net.d` and `/opt/cni/bin`; without
these two values the CNI DaemonSet waits forever and ztunnel cannot start.

`GatewayClass` should include:

```text
istio
istio-waypoint
```

Do **not** separately install a second `istio-ingressgateway` Helm release. The
`mall` chart's `Gateway` creates `mall-gateway-istio` and k3s servicelb assigns
`10.211.55.29`. A second ingress gateway competes for host port 80 and remains
`pending`.

`ENABLE_INGRESS_WAYPOINT_ROUTING=true`, plus the
`istio.io/ingress-use-waypoint: "true"` label on the gateway/admin Services
(the chart adds it), makes north-south ingress go through the waypoint instead
of directly to the backend. Without them the ingress gateway can connect to the
app endpoint but the app's ztunnel rejects the HBONE path.

## 6. Build and import images

On the Mac, with Docker running:

```bash
make k8s-images IMAGE_REGISTRY=mall IMAGE_TAG=dev
make k8s-import K8S_REMOTE=root@10.211.55.29
```

The app images in this validation were built as Linux binaries
(`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`) and imported via
`k3s ctr -n k8s.io images import -`. Do not import macOS binaries into Linux
containers.

Infra images (`mysql:8.4`, `redis:8.10.3-alpine`, `nats:2.10-alpine`,
`quay.io/coreos/etcd:v3.5.17`, `yedf/dtm:latest`, `mall/migrate:dev`) must also
be imported or pullable through the proxy.

## 7. Deploy

```bash
kubectl apply -f deploy/k8s/manifests/namespaces.yaml
helm upgrade --install mall-infra deploy/k8s/charts/infra \
  -n mall-data -f deploy/k8s/charts/infra/values-single-node.yaml

helm upgrade --install mall deploy/k8s/charts/mall \
  -n mall -f deploy/k8s/charts/mall/values-single-node.yaml

kubectl get pods -n mall
kubectl get pods -n mall-data
kubectl get gateway,httproute -A
```

The `mall` chart creates:

- `mall-waypoint`
- `mall-gateway` and `mall-gateway-istio`
- STRICT mTLS and L4/L7 AuthorizationPolicy
- NetworkPolicy default deny

Ingress is available at:

```bash
curl -H 'Host: mall.local' http://10.211.55.29/api/v1/products
```

## 8. Smoke

```bash
KUBECONFIG=~/.kube/mall-k3s.yaml make mesh-smoke
```

The smoke test checks deployments, migration Job, no application sidecars,
STRICT mTLS, port-forwarded C-end/DTM flow, and the ingress product endpoint.

If `istiod`, `ztunnel`, or the waypoint was restarted, restart the application
deployments once after they are Ready. In-pod ztunnel proxies are created when
an ambient pod starts; existing pods can be left without a working `15008`
listener (symptom: ingress 503, waypoint log
`connecting to <pod>:15008 ... Connection refused`).

```bash
kubectl -n mall rollout restart deployment
kubectl -n mall rollout status deployment --timeout=180s
```

## 9. Local debugging

```bash
export KUBECONFIG=$HOME/.kube/mall-k3s.yaml

kubectl -n mall port-forward svc/gateway 8080:8080
kubectl -n mall port-forward svc/admin 8081:8081
kubectl -n mall logs -f deploy/order
kubectl -n mall exec -it deploy/order -- sh

istioctl ztunnel-config workloads
kubectl get peerauthentication -n mall -o yaml
```

The Mac workspace remains the source of truth. No DSH installation is needed
inside the VM.
