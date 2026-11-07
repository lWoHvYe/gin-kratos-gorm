# Istio Ambient install notes

Pin the Istio version in `version` and use the matching `istioctl` binary.

Typical sequence (verify against the pinned version's official Ambient guide):

```bash
export ISTIO_VERSION="$(tr -d '[:space:]' < deploy/k8s/istio/version)"
# install istioctl matching ISTIO_VERSION, then:
istioctl install --set profile=ambient --skip-confirmation

# Gateway API CRDs are required for Gateway/HTTPRoute and the waypoint.
# Use the Gateway API release bundled/tested with the pinned Istio version.

# Confirm the ambient data plane:
kubectl get pods -n istio-system
kubectl get crd | grep -E 'gateways|httproutes|peerauthentications|authorizationpolicies'
```

The chart expects:

- `GatewayClass: istio` for north-south ingress;
- `GatewayClass: istio-waypoint` for `mall-waypoint`;
- an ingress gateway deployment/service in `istio-system`;
- the Gateway API CRDs installed.

The `mall` namespace labels and waypoint are created by
`deploy/k8s/manifests/namespaces.yaml` and the `mall` chart. Do not add
`istio-injection=enabled`; Ambient does not use application sidecars.
