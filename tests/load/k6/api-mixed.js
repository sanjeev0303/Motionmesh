import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '1m', target: 500 },
    { duration: '3m', target: 500 },
    { duration: '1m', target: 0 },
  ],
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  if (Math.random() < 0.8) {
    const res = http.get(`${BASE_URL}/api/v1/videos`);
    check(res, { 'status is 200': (r) => r.status === 200 });
  } else {
    const payload = JSON.stringify({ title: 'Test Video', description: 'Mixed workload test' });
    const res = http.post(`${BASE_URL}/api/v1/videos`, payload, {
      headers: { 'Content-Type': 'application/json' },
    });
    check(res, { 'status is 201': (r) => r.status === 201 });
  }
  sleep(1);
}
