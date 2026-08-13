const { Motionmesh } = require('@motionmesh/sdk');

// Ensure we have deterministic data
const fs = require('fs');
const path = require('path');
const dataPath = path.join(__dirname, '../../tests/load/k6/data.json');
let data = {};
try {
  data = JSON.parse(fs.readFileSync(dataPath, 'utf8'));
} catch (e) {
  console.error("data.json not found or invalid JSON");
  process.exit(1);
}

if (!data.api_keys || data.api_keys.length === 0) {
  console.error("Missing api_keys in data.json");
  process.exit(1);
}

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const NUM_REQUESTS = parseInt(process.env.NUM_REQUESTS || "1000");
const CONCURRENCY = parseInt(process.env.CONCURRENCY || "50");
const API_KEY = data.api_keys[0];

const client = new Motionmesh({
  apiKey: API_KEY,
  baseURL: BASE_URL,
});

async function runBenchmark() {
  console.log(`Starting Official SDK Benchmark...`);
  console.log(`Base URL: ${BASE_URL}`);
  console.log(`Target: ${NUM_REQUESTS} requests at concurrency ${CONCURRENCY}`);

  let completed = 0;
  let errors = 0;
  let latencies = [];
  const start = Date.now();

  async function worker() {
    while (true) {
      if (completed + errors >= NUM_REQUESTS) break;
      const reqStart = Date.now();
      try {
        await client.videos.list({ limit: 10 });
        completed++;
        latencies.push(Date.now() - reqStart);
      } catch (err) {
        errors++;
      }
    }
  }

  const workers = [];
  for (let i = 0; i < CONCURRENCY; i++) {
    workers.push(worker());
  }

  await Promise.all(workers);
  
  const end = Date.now();
  const totalTimeSec = (end - start) / 1000;
  const rps = (completed + errors) / totalTimeSec;

  latencies.sort((a, b) => a - b);
  const p50 = latencies[Math.floor(latencies.length * 0.50)] || 0;
  const p95 = latencies[Math.floor(latencies.length * 0.95)] || 0;
  const p99 = latencies[Math.floor(latencies.length * 0.99)] || 0;

  console.log("\n--- Official SDK Benchmark Results ---");
  console.log(`Total Requests: ${completed + errors}`);
  console.log(`Successful:     ${completed}`);
  console.log(`Failed:         ${errors}`);
  console.log(`Time Elapsed:   ${totalTimeSec.toFixed(2)}s`);
  console.log(`RPS:            ${rps.toFixed(2)} req/s`);
  console.log(`Latency p50:    ${p50} ms`);
  console.log(`Latency p95:    ${p95} ms`);
  console.log(`Latency p99:    ${p99} ms`);

  // Write JSON output for aggregators
  const results = {
    type: "official_sdk",
    total_requests: completed + errors,
    successful: completed,
    failed: errors,
    time_elapsed_s: totalTimeSec,
    rps: rps,
    latency: {
      p50, p95, p99
    }
  };
  fs.writeFileSync(path.join(__dirname, 'sdk_benchmark_results.json'), JSON.stringify(results, null, 2));
}

runBenchmark().catch(console.error);
