import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 }, 
    { duration: '5m', target: 100 }, 
    { duration: '2m', target: 500 }, 
    { duration: '5m', target: 500 }, 
    { duration: '2m', target: 1000 }, 
    { duration: '5m', target: 1000 }, 
    { duration: '2m', target: 0 }, 
  ],
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE_URL}/api/v1/videos`);
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(1);
}
