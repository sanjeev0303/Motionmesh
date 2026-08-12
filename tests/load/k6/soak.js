import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 400 },
    { duration: '3h', target: 400 },
    { duration: '2m', target: 0 },
  ],
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/videos`);
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(1);
}
