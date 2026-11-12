import http from 'k6/http'
import { check, sleep } from 'k6'
import { Counter, Trend } from 'k6/metrics'

const base = __ENV.BASE_URL || 'http://127.0.0.1:18080'
const activityID = __ENV.ACTIVITY_ID || '1'
const username = __ENV.USERNAME || 'demo'
const password = __ENV.PASSWORD || 'user123'
const vus = Number(__ENV.VUS || '1000')
const duration = __ENV.DURATION || '10s'

const accepted = new Counter('seckill_accepted')
const soldOut = new Counter('seckill_sold_out')
const rateLimited = new Counter('seckill_rate_limited')
const admissionLatency = new Trend('seckill_admission_latency', true)

export const options = {
  scenarios: {
    admission: {
      executor: 'constant-vus',
      vus,
      duration,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
    seckill_admission_latency: ['p(99)<200'],
  },
}

export function setup() {
  const login = http.post(`${base}/api/v1/auth/login`, JSON.stringify({ username, password }), {
    headers: { 'Content-Type': 'application/json' },
  })
  check(login, { 'login 200': (r) => r.status === 200 })
  const token = login.json('access_token')
  return { token }
}

export default function (data) {
  const res = http.post(`${base}/api/v1/seckill/activities/${activityID}/reserve`, JSON.stringify({}), {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${data.token}`,
      'Idempotency-Key': `${__VU}-${__ITER}`,
    },
    tags: { endpoint: 'seckill-reserve' },
  })
  admissionLatency.add(res.timings.duration)
  check(res, { 'expected admission result': (r) => [200, 202, 409, 410, 429].includes(r.status) })
  if ([200, 202].includes(res.status)) accepted.add(1)
  if ([409, 410].includes(res.status)) soldOut.add(1)
  if (res.status === 429) rateLimited.add(1)
  sleep(0.1)
}
