# Istio Ambient target

## Why Ambient

Ambient uses a per-node ztunnel for L4 identity and mTLS, with a namespace
waypoint for L7 authorization and traffic policy. It avoids application
sidecars and keeps the Go services unchanged at the transport level.

## Boundaries

- `mall` is enrolled in Ambient, with `mall-waypoint`.
- `mall-data` is not enrolled; MySQL/Redis/NATS/etcd are controlled by
  NetworkPolicy only.
- DTM runs in `mall` because it calls application branches.
- etcd is optional and used only for the configuration overlay, never for
  service discovery.

## Service discovery

The application no longer uses Kratos etcd discovery, `GRPC_ENDPOINT`, or
`discovery:///`. Services resolve each other through Kubernetes DNS:

```text
inventory.mall.svc.cluster.local:9003
order.mall.svc.cluster.local:9006
dtm.mall.svc.cluster.local:36790
```

DTM and TCC branch URLs use direct `host:port` targets. This removes the
dual-discovery path and lets Ambient own mTLS/waypoint policy.

## Security

- `PeerAuthentication` is `STRICT`.
- `AuthorizationPolicy` default-denies and allows ingress paths plus mall
  internal service ports.
- NetworkPolicy default-denies, with explicit DNS, `mall-data`, ingress gateway
  and node-probe allowances.
- No application NodePorts/HostPorts.

## Verification

Runtime verification is intentionally deferred until the user's single-node
K8s cluster is available. Before then, use:

```bash
make k8s-lint
make k8s-render
make verify
```

After deployment:

```bash
make mesh-smoke
```

Expected checks:

- all pods Ready;
- no `istio-proxy` sidecars;
- `PeerAuthentication` mode `STRICT`;
- waypoint Ready;
- ingress reaches gateway/admin;
- C-end checkout and DTM Saga/TCC work;
- L7 deny/allow behavior is visible in waypoint metrics.
