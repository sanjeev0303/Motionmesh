import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';

export const options = {
  vus: 50,
  duration: '1m',
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

const apiKeys = new SharedArray('api keys', function () {
  return JSON.parse(open('./data.json')).api_keys;
});

export default function () {
  const apiKey = apiKeys[exec.vu.idInTest % apiKeys.length];
  const payload = JSON.stringify({ status: 'processing', progress: 50 });
  const res = http.post(`${BASE_URL}/api/v1/jobs/123/status`, payload, {
    headers: { 
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${apiKey}`,
    },
  });
  check(res, { 'status is 200 or 404': (r) => r.status === 200 || r.status === 404 });
  sleep(2);
}
