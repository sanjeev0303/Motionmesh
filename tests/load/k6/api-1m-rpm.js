import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';

export const options = {
  scenarios: {
    constant_request_rate: {
      executor: 'constant-arrival-rate',
      rate: 16667,          // 1,000,000 / 60 ≈ 16,667 RPS
      timeUnit: '1s',
      duration: '2m',       // 2-minute sustained run for steady-state measurement
      preAllocatedVUs: 10000,
      maxVUs: 25000,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],   // <1% error rate target
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
  if (rand < 0.70) {
    // 70%: list videos (hot read path — should hit Redis/LRU)
    const res = http.get(`${BASE_URL}/v1/videos?limit=10`, params);
    check(res, { 'list videos 200': (r) => r.status === 200 });
  } else if (rand < 0.90) {
    // 20%: list objects in a specific bucket (tests keyset pagination + authz)
    const res = http.get(`${BASE_URL}/v1/buckets?limit=5`, params);
    check(res, { 'list buckets 200': (r) => r.status === 200 });
  } else {
    // 10%: create a video (write path — exercises DB insert + outbox)
    const payload = JSON.stringify({
      title: `load-test-${exec.vu.idInTest}-${Date.now()}`,
      filename: 'load-test.mp4',
    });
    const res = http.post(`${BASE_URL}/v1/videos`, payload, params);
    check(res, { 'create video 2xx': (r) => r.status === 200 || r.status === 201 });
  }
}
