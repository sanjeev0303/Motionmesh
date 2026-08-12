import http from 'k6/http';
import { check } from 'k6';

export const options = {
  scenarios: {
    constant_request_rate: {
      executor: 'constant-arrival-rate',
      rate: 16666,
      timeUnit: '1s',
      duration: '1m',
      preAllocatedVUs: 5000,
      maxVUs: 20000,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/videos?limit=10`);
  check(res, { 'status is 200': (r) => r.status === 200 });
}
