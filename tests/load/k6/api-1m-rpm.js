import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';

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

const apiKeys = new SharedArray('api keys', function () {
  return JSON.parse(open('./data.json')).api_keys;
});

export default function () {
  const apiKey = apiKeys[exec.vu.idInTest % apiKeys.length];
  const params = {
    headers: {
      'Authorization': `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
    },
  };

  const rand = Math.random();
  if (rand < 0.8) {
    // 80% read traffic
    const res = http.get(`${BASE_URL}/api/v1/videos?limit=10`, params);
    check(res, { 'status is 200': (r) => r.status === 200 });
  } else {
    // 20% write traffic
    const payload = JSON.stringify({
      bucket_id: 'default', // Using a default bucket name or ID for the test
      title: 'Load Test Video',
      filename: 'load-test.mp4'
    });
    const res = http.post(`${BASE_URL}/api/v1/videos`, payload, params);
    // Might fail with 400 or 404 if bucket doesn't exist, just check status
    check(res, { 'status is 200 or 201': (r) => r.status === 200 || r.status === 201 });
  }
}
