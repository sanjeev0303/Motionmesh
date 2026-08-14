import { motionmesh } from '@motionmesh/sdk';
import { performance } from 'perf_hooks';

const API_URL = process.env.API_URL || 'http://localhost:8080';
const API_KEY = process.env.API_KEY || 'test_api_key';
const RPS_TARGET = parseInt(process.env.RPS_TARGET || '1000', 10);
const DURATION_SECONDS = parseInt(process.env.DURATION_SECONDS || '10', 10);
const CLIENT_TYPE = process.env.CLIENT_TYPE || 'sdk'; // 'sdk' or 'http'
const MAX_CONCURRENCY = parseInt(process.env.MAX_CONCURRENCY || '500', 10);
const MAX_SAMPLES = 100000;

const client = new motionmesh({
  apiKey: API_KEY,
  baseURL: API_URL
});

let currentConcurrency = 0;
let requestedCount = 0;
let sentCount = 0;
let completedCount = 0;
let successfulCount = 0;
let failedCount = 0;
let droppedCount = 0;

let latencies = [];
let totalLatencyMs = 0;

async function makeRequest() {
  requestedCount++;
  
  if (currentConcurrency >= MAX_CONCURRENCY) {
    droppedCount++;
    return;
  }
  
  currentConcurrency++;
  sentCount++;
  
  const start = process.hrtime.bigint();
  try {
    if (CLIENT_TYPE === 'sdk') {
      await client.videos.list({ limit: 5 });
      successfulCount++;
    } else {
      const res = await fetch(`${API_URL}/v1/videos?limit=5`, {
        headers: { 'Authorization': `Bearer ${API_KEY}` }
      });
      if (res.ok) {
        successfulCount++;
      } else {
        failedCount++;
      }
    }
  } catch (err) {
    failedCount++;
  } finally {
    const end = process.hrtime.bigint();
    const latencyMs = Number(end - start) / 1000000;
    
    completedCount++;
    totalLatencyMs += latencyMs;
    
    // Reservoir sampling
    if (latencies.length < MAX_SAMPLES) {
      latencies.push(latencyMs);
    } else {
      const r = Math.floor(Math.random() * completedCount);
      if (r < MAX_SAMPLES) {
        latencies[r] = latencyMs;
      }
    }
    
    currentConcurrency--;
  }
}

async function runBenchmark() {
  console.log(`Starting ${CLIENT_TYPE.toUpperCase()} Benchmark targeting ${RPS_TARGET} RPS for ${DURATION_SECONDS} seconds...`);
  
  let running = true;
  setTimeout(() => {
    running = false;
  }, DURATION_SECONDS * 1000);

  const promises = [];
  
  // Token bucket arrival rate controller (smooth scheduler)
  const intervalMs = 10; // 10ms ticks
  const tokensPerTick = RPS_TARGET / (1000 / intervalMs);
  let tokenBucket = 0;

  while (running) {
    const tickStart = performance.now();
    tokenBucket += tokensPerTick;
    
    while (tokenBucket >= 1) {
      promises.push(makeRequest());
      tokenBucket -= 1;
    }
    
    const tickDuration = performance.now() - tickStart;
    const sleepTime = intervalMs - tickDuration;
    
    if (sleepTime > 0) {
      await new Promise(resolve => setTimeout(resolve, sleepTime));
    } else {
      // Yield to event loop even if falling behind
      await new Promise(resolve => setImmediate(resolve));
    }
  }

  console.log("Waiting for requests to finish...");
  await Promise.all(promises);

  latencies.sort((a, b) => a - b);
  const p50 = latencies[Math.floor(latencies.length * 0.5)] || 0;
  const p95 = latencies[Math.floor(latencies.length * 0.95)] || 0;
  const p99 = latencies[Math.floor(latencies.length * 0.99)] || 0;
  const avg = completedCount > 0 ? totalLatencyMs / completedCount : 0;
  
  const memUsage = process.memoryUsage();

  console.log(`\n--- Benchmark Results ---`);
  console.log(`Requested:  ${requestedCount} (${Math.round(requestedCount / DURATION_SECONDS)} req/s)`);
  console.log(`Sent:       ${sentCount} (${Math.round(sentCount / DURATION_SECONDS)} req/s)`);
  console.log(`Completed:  ${completedCount}`);
  console.log(`Successful: ${successfulCount}`);
  console.log(`Failed:     ${failedCount}`);
  console.log(`Dropped:    ${droppedCount}`);
  
  if (droppedCount > sentCount * 0.1) {
    console.log(`\n[WARNING] LOAD_GENERATOR_SATURATED - SDK benchmark fell behind or was throttled by MAX_CONCURRENCY.`);
  }

  if (latencies.length > 0) {
    console.log(`Avg Latency:   ${avg.toFixed(2)}ms`);
    console.log(`p50 Latency:   ${p50.toFixed(2)}ms`);
    console.log(`p95 Latency:   ${p95.toFixed(2)}ms`);
    console.log(`p99 Latency:   ${p99.toFixed(2)}ms`);
  }
  console.log('--- Resource Usage ---');
  console.log(`Memory (RSS): ${Math.round(memUsage.rss / 1024 / 1024)} MB`);
  console.log(`Memory (HeapUsed): ${Math.round(memUsage.heapUsed / 1024 / 1024)} MB`);
}

runBenchmark().catch(console.error);
