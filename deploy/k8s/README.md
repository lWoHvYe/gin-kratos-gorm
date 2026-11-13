# Kubernetes + Istio Ambient deployment

This directory targets **Kubernetes + Istio Ambient**. It is implementation-only
until a single-node cluster is available; `make k8s-render` and `make k8s-lint`
work without a cluster.

For the Debian VM + k3s installation path, see
[`install/debian-k3s.md`](install/debian-k3s.md).

## Assumptions

- Kubernetes 1.30+ with a default StorageClass.
- Istio Ambient installed with a pinned supported version.
- Gateway API CRDs installed.
- `kubectl` and `helm` configured against the target cluster.
- The single-node profile uses one replica per workload and one PVC per data
  component.

## 1. Build images

```bash
make k8s-images IMAGE_REGISTRY=mall IMAGE_TAG=dev
```

For a local registry/cluster, push or load the images using the cluster's
normal image-distribution mechanism.

## 2. Install namespaces

```bash
kubectl apply -f deploy/k8s/manifests/namespaces.yaml
```

`mall` is labeled:

```text
istio.io/dataplane-mode=ambient
istio.io/use-waypoint=mall-waypoint
```

`mall-data` is deliberately outside Ambient.

## 3. Install Istio Ambient

Install the pinned Istio release and the Gateway API CRDs before this chart.
The required components are:

- istiod
- istio-cni
- ztunnel
- Gateway API
- a `Gateway` object using `GatewayClass: istio`; the chart creates the
  ingress gateway service

For k3s, install with the CNI paths in `install/debian-k3s.md`. Do not install a
second ingress gateway; the `mall` chart's `Gateway` creates it.

## 4. Deploy

```bash
make k8s-deploy
```

This installs:

- `mall-infra` in `mall-data`: MySQL, Redis, NATS, etcd, migration Job.
- `mall` in `mall`: 12 services (including seckill), DTM, waypoint, STRICT mTLS,
  L7 policies, Gateway API route.

The migration Job is a Helm post-install/pre-upgrade hook and is safe to
rerun.

## 5. Verify

After the user's cluster is ready:

```bash
make mesh-smoke
```

The smoke script waits for pods, checks Ambient enrollment and STRICT mTLS,
port-forwards gateway/admin, and runs the existing C-end/DTM smoke.

## 6. Production HA overlay

The charts ship an opt-in HA overlay; the default profile above is unchanged.

```bash
make k8s-ha-verify     # lint + render + topology assertions, no cluster needed
make k8s-render-ha     # /tmp/{mall,mall-infra}-ha-rendered.yaml
```

- `charts/infra/values-ha.yaml`: Redis Sentinel (1 master / 2 replicas /
  3 sentinels), NATS 3-node JetStream cluster, external MySQL, PDBs.
- `charts/mall/values-ha.yaml`: 2-4 replicas per service, Sentinel/stream env,
  Secret-based MySQL credentials, pool caps, PDBs.

Read [docs/production-ha.md](../../docs/production-ha.md) before deploying it:
it explains the secret prerequisites, connection-limit formula, failover drills
and what is deliberately delegated to a managed MySQL/Redis.

## Browser access

The frontends are at `http://mall.local/` and `/admin/` when `mall.local` resolves to the Gateway LoadBalancer. See [../../docs/access.md](../../docs/access.md) for hosts and port-forward instructions.
