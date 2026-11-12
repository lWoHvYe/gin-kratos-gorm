#!/usr/bin/env bash
set -euo pipefail
IP="${K8S_IP:-10.211.55.29}"
HOST="${MALL_HOST:-mall.local}"
cat <<INFO
Mall access
  hosts entry: $IP $HOST
  storefront : http://$HOST/
  admin      : http://$HOST/admin/

Direct port-forward fallback:
  kubectl -n mall port-forward svc/storefront-web 13000:3000
  kubectl -n mall port-forward svc/admin-web 18082:8080
  storefront : http://127.0.0.1:13000/
  admin      : http://127.0.0.1:18082/admin/

Demo: admin/admin123, demo/user123
INFO
