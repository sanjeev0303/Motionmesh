import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 20,
  duration: '30s',
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/sdk/config`);
  check(res, { 'status is 200 or 404': (r) => r.status === 200 || r.status === 404 });
  sleep(1);
}
