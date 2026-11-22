#!/bin/sh
# Structural guard for the after-sale backlog alerts. This does not run
# Prometheus; it verifies the rule names, expressions, severity and durations
# against the metric names exported by the after-sales workers.
set -eu
root="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
file="$root/deploy/compose/observability/alerts.yml"

ruby - "$file" <<'RUBY'
require "yaml"
path = ARGV.fetch(0)
rules = YAML.load_file(path).fetch("groups").flat_map { |g| g.fetch("rules") }
required = {
  "MallAfterSaleRestockBacklog" => ["mall_aftersale_restock_backlog > 0", "10m", "warning"],
  "MallAfterSaleRestockStalled" => ["mall_aftersale_restock_oldest_seconds > 300", "5m", "critical"],
  "MallAfterSaleRefundBacklog" => ["mall_aftersale_refund_backlog > 0", "10m", "warning"],
  "MallAfterSaleRefundStalled" => ["mall_aftersale_refund_oldest_seconds > 300", "5m", "critical"],
}
by_name = rules.to_h { |r| [r["alert"], r] }
required.each do |name, (expr, duration, severity)|
  rule = by_name[name] or abort("missing alert #{name}")
  abort("bad expr for #{name}: #{rule["expr"]}") unless rule["expr"] == expr
  abort("bad for for #{name}: #{rule["for"]}") unless rule["for"] == duration
  abort("bad severity for #{name}") unless rule.dig("labels", "severity") == severity
end
puts "[alerts] OK (#{required.length} after-sale backlog rules)"
RUBY

if command -v docker >/dev/null 2>&1; then
  docker run --rm --entrypoint promtool     -v "$root/deploy/compose/observability:/work" -w /work     "${PROMTOOL_IMAGE:-prom/prometheus:v3.1.0}" test rules alerts_test.yml
else
  echo "[alerts] promtool unavailable; structural check only" >&2
fi
