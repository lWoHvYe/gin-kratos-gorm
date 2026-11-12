# k6 seckill load tests

The admission endpoint is introduced in S2. This harness is checked in during S0
so the target envelope can be tested as soon as the endpoint exists.

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
