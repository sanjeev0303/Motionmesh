import { Motionmesh } from '@motionmesh/sdk';
import { performance } from 'perf_hooks';

const API_URL = process.env.API_URL || 'http://localhost:8080';
const API_KEY = process.env.API_KEY || 'test_api_key';
const RPS_TARGET = parseInt(process.env.RPS_TARGET || '1000', 10);
const DURATION_SECONDS = parseInt(process.env.DURATION_SECONDS || '10', 10);

const client = new Motionmesh({
  apiKey: API_KEY,
  baseURL: API_URL
});

let successCount = 0;
let errorCount = 0;
let latencies = [];
const MAX_SAMPLES = 100000;
let totalRequests = 0;
let totalLatencyMs = 0;

// Semaphore to limit concurrency (unbounded promises can cause OOM)
const MAX_CONCURRENCY = RPS_TARGET * 2;
let currentConcurrency = 0;

async function makeRequest() {
  if (currentConcurrency >= MAX_CONCURRENCY) {
    // Drop or delay? For a load generator, if we hit max concurrency, we're falling behind.
    errorCount++;
    return;
  }
  
  currentConcurrency++;
  const start = process.hrtime.bigint();
  try {
    // A standard high-volume endpoint
    await client.videos.list({ limit: 5 });
    successCount++;
  } catch (err) {
    errorCount++;
  } finally {
    const end = process.hrtime.bigint();
    const latencyMs = Number(end - start) / 1000000;
    
    totalRequests++;
    totalLatencyMs += latencyMs;
    
    // Reservoir sampling
    if (latencies.length < MAX_SAMPLES) {
      latencies.push(latencyMs);
    } else {
      const r = Math.floor(Math.random() * totalRequests);
      if (r < MAX_SAMPLES) {
        latencies[r] = latencyMs;
      }
    }
    
    currentConcurrency--;
  }
}

async function runBenchmark() {
  console.log(`Starting SDK Benchmark targeting ${RPS_TARGET} RPS for ${DURATION_SECONDS} seconds...`);
  
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
  const avg = totalRequests > 0 ? totalLatencyMs / totalRequests : 0;
  
  const memUsage = process.memoryUsage();

  console.log('--- SDK Benchmark Results ---');
  console.log(`Total Requests: ${successCount + errorCount}`);
  console.log(`Success: ${successCount}`);
  console.log(`Errors: ${errorCount}`);
  console.log(`Avg Latency: ${avg.toFixed(2)}ms`);
  console.log(`p50 Latency: ${p50.toFixed(2)}ms`);
  console.log(`p95 Latency: ${p95.toFixed(2)}ms`);
  console.log(`p99 Latency: ${p99.toFixed(2)}ms`);
  console.log('--- Resource Usage ---');
  console.log(`Memory (RSS): ${Math.round(memUsage.rss / 1024 / 1024)} MB`);
  console.log(`Memory (HeapUsed): ${Math.round(memUsage.heapUsed / 1024 / 1024)} MB`);
}

runBenchmark().catch(console.error);
