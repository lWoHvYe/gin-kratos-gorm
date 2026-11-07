# Debian VM + single-node k3s + Istio Ambient

This is the recommended local target when a Debian VM with at least **4 vCPU /
8 GiB RAM / 50 GiB disk** is available. The Mac remains the development and
control machine; the VM runs Kubernetes.

## Why VM rather than Docker Desktop

Istio Ambient depends on Linux CNI/ztunnel networking. A native Debian/k3s node
is more predictable and closer to production than a nested kind cluster inside
Docker Desktop. The Mac still gets a normal kubeconfig, `kubectl`, `helm`,
`istioctl`, port-forward and IDE workflow.

## 1. Prepare the VM

On the Mac:

```bash
ssh-copy-id root@10.211.55.29
ssh root@10.211.55.29
```

On the VM:

```bash
apt update
apt install -y curl ca-certificates jq
hostnamectl set-hostname mall-k8s

# Confirm resources before installing.
nproc
free -h
df -h /
```

If a firewall is enabled, allow:

```text
6443/tcp   Kubernetes API
80/tcp     ingress
443/tcp    ingress TLS (optional)
30000-32767/tcp  NodePort (temporary/debug)
```

## 2. Install k3s

```bash
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --write-kubeconfig-mode 644 \
  --tls-san 10.211.55.29 \
  --node-external-ip 10.211.55.29 \
  --disable traefik" sh -
```

We disable Traefik so Istio owns ingress; k3s servicelb remains available for
the Istio ingress `LoadBalancer`.

Verify:

```bash
systemctl status k3s
k3s kubectl get nodes -o wide
k3s kubectl get storageclass
```

## 3. Use the cluster from the Mac

```bash
mkdir -p ~/.kube
scp root@10.211.55.29:/etc/rancher/k3s/k3s.yaml ~/.kube/mall-k3s.yaml
sed -i '' 's/127.0.0.1/10.211.55.29/' ~/.kube/mall-k3s.yaml
export KUBECONFIG=$HOME/.kube/mall-k3s.yaml
kubectl get nodes
```

The project's `deploy/k8s` Make targets use the current `KUBECONFIG`.

## 4. Install Istio Ambient

Pin the version in `deploy/k8s/istio/version` and use the matching `istioctl`.

```bash
export ISTIO_VERSION="$(tr -d '[:space:]' < deploy/k8s/istio/version)"
# install istioctl ${ISTIO_VERSION} on the Mac, then:
istioctl install --set profile=ambient --skip-confirmation
kubectl get pods -n istio-system
```

Install Gateway API CRDs matching the pinned Istio release, then the ingress
gateway. One supported route is the official Istio gateway Helm chart:

```bash
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update
helm upgrade --install istio-ingressgateway istio/gateway -n istio-system --create-namespace
```

Confirm `GatewayClass` resources:

```bash
kubectl get gatewayclass
# expect istio and istio-waypoint
```

The chart expects `istio` for north-south and `istio-waypoint` for the mall
namespace waypoint.

## 5. Build and import images

On the Mac:

```bash
make k8s-images IMAGE_REGISTRY=mall IMAGE_TAG=dev
make k8s-import K8S_REMOTE=root@10.211.55.29
```

`k8s-import` copies each image into the k3s containerd namespace `k8s.io`, so
the charts use `imagePullPolicy: IfNotPresent` without a registry.

For iterative development:

```bash
make k8s-images IMAGE_REGISTRY=mall IMAGE_TAG=dev
# import only the changed image
docker save mall/order:dev | ssh root@10.211.55.29 'k3s ctr -n k8s.io images import -'
kubectl -n mall rollout restart deployment/order
```

A local registry can replace the import step later, but is not required.

## 6. Deploy

```bash
kubectl apply -f deploy/k8s/manifests/namespaces.yaml
make k8s-deploy
kubectl get pods -n mall
kubectl get pods -n mall-data
kubectl get gateway,httproute -A
```

Set Mac hosts resolution:

```bash
# /etc/hosts
10.211.55.29 mall.local
```

Then browse `http://mall.local/api/v1/products` or use
`curl -H 'Host: mall.local' http://10.211.55.29/api/v1/products`.

## 7. Local debugging

```bash
export KUBECONFIG=$HOME/.kube/mall-k3s.yaml

kubectl -n mall port-forward svc/gateway 8080:8080
kubectl -n mall port-forward svc/admin 8081:8081
kubectl -n mall logs -f deploy/order
kubectl -n mall exec -it deploy/order -- sh

istioctl ztunnel-config workloads -n mall
kubectl get peerauthentication -n mall -o yaml
```

The Mac workspace remains the source of truth. The VM only runs the cluster;
there is no need to install DSH inside the VM.

## 8. Smoke and acceptance

```bash
make mesh-smoke
```

The smoke script checks pod readiness, Ambient enrollment/no sidecars, STRICT
mTLS, port-forwards gateway/admin, runs the DTM/C-end smoke, and checks
ingress.

Runtime acceptance remains deliberately deferred until the VM is available.
