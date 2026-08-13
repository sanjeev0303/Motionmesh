import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';

const RPS_TARGET = __ENV.RPS_TARGET ? parseInt(__ENV.RPS_TARGET) : 16667;

export const options = {
  scenarios: {
    constant_request_rate: {
      executor: 'constant-arrival-rate',
      rate: RPS_TARGET,     // configurable RPS
      timeUnit: '1s',
      duration: '2m',       // 2-minute sustained run for steady-state measurement
      preAllocatedVUs: Math.min(10000, RPS_TARGET),
      maxVUs: Math.min(25000, RPS_TARGET * 2),
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    http_req_failed: ['rate<0.01'],   // <1% error rate target
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

let data = {};
try {
  data = JSON.parse(open('./data.json'));
} catch (e) {
  throw new Error("data.json not found or invalid JSON");
}

if (!data.api_keys || data.api_keys.length === 0) throw new Error("Missing api_keys in data.json");
if (!data.video_ids || data.video_ids.length === 0) throw new Error("Missing video_ids in data.json");
if (!data.bucket_ids || data.bucket_ids.length === 0) throw new Error("Missing bucket_ids in data.json");

const apiKeys = new SharedArray('api keys', function () {
  return data.api_keys;
});
const videoIds = new SharedArray('video ids', function () {
  return data.video_ids;
});
const bucketIds = new SharedArray('bucket ids', function () {
  return data.bucket_ids;
});

export function setup() {
  const res = http.get(`${BASE_URL}/health`);
  if (res.status !== 200) {
    throw new Error(`API is not healthy, status: ${res.status}`);
  }
  
  // Verify first API key works
  const authRes = http.get(`${BASE_URL}/v1/videos?limit=1`, {
    headers: { 'Authorization': `Bearer ${apiKeys[0]}` }
  });
  if (authRes.status !== 200) {
    throw new Error(`Pre-flight auth check failed. Expected 200, got ${authRes.status}`);
  }
}

export default function () {
  const vuIndex = exec.scenario.iterationInTest;
  const apiKey = apiKeys[vuIndex % apiKeys.length];
  const videoId = videoIds[vuIndex % videoIds.length];
  const bucketId = bucketIds[vuIndex % bucketIds.length];
  
  const params = {
    headers: {
      'Authorization': `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
      'X-Mock-Billing': 'true', // Exclude Stripe calls from API capacity test
    },
  };

  const rand = Math.random();
  if (rand < 0.30) {
    const res = http.get(`${BASE_URL}/v1/videos?limit=10`, params);
    check(res, { 'list videos 200': (r) => r.status === 200 });
  } else if (rand < 0.50) {
    const res = http.get(`${BASE_URL}/v1/videos/${videoId}`, params);
    check(res, { 'video detail 200': (r) => r.status === 200 });
  } else if (rand < 0.65) {
    const res = http.get(`${BASE_URL}/v1/videos/${videoId}/playback`, params);
    check(res, { 'playback 200': (r) => r.status === 200 });
  } else if (rand < 0.75) {
    const res = http.get(`${BASE_URL}/v1/jobs`, params);
    check(res, { 'list jobs 200': (r) => r.status === 200 });
  } else if (rand < 0.85) {
    const res = http.get(`${BASE_URL}/v1/buckets`, params);
    check(res, { 'list buckets 200': (r) => r.status === 200 });
  } else if (rand < 0.90) {
    const res = http.get(`${BASE_URL}/v1/buckets/${bucketId}/objects`, params);
    check(res, { 'list objects 200': (r) => r.status === 200 });
  } else if (rand < 0.95) {
    // 5%: branding
    const res = http.get(`${BASE_URL}/v1/branding`, params);
    check(res, { 'branding 200': (r) => r.status === 200 });
  } else {
    // 5%: create a video (write path — exercises DB insert + outbox)
    const payload = JSON.stringify({
      title: `load-test-${exec.vu.idInTest}-${Date.now()}`,
      filename: 'load-test.mp4',
    });
    const res = http.post(`${BASE_URL}/v1/videos`, payload, params);
    check(res, { 'create video 2xx': (r) => r.status === 200 || r.status === 201 });
  }
}
