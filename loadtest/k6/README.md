# k6 seckill load tests

The admission endpoint exists in S2. This harness is checked in during S0 and
now targets the live endpoint. It still needs a strategy for provisioning
distinct authenticated users before it can represent a 10k-user sale; do not
treat a single-token run as capacity evidence.

```bash
k6 run \
  -e BASE_URL=http://127.0.0.1:18080 \
  -e ACTIVITY_ID=1 \
  -e VUS=10000 \
  -e DURATION=10s \
  loadtest/k6/seckill-admission.js
```

The expected business outcome for a 100-unit sale is approximately 100 accepted
qualifications and the rest rejected with a business status, not 5xx/timeouts.
Run against a production-like Redis/NATS/MySQL topology; single-node dev
results are not capacity evidence.
