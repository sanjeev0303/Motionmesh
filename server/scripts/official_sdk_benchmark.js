const { Motionmesh } = require('@motionmesh/sdk');
const fs = require('fs');
const path = require('path');
const { performance } = require('perf_hooks');

// Reservoir Sampling for Bounded Memory Latency Tracking
class ReservoirSampler {
  constructor(capacity = 1024) {
    this.capacity = capacity;
    this.reservoir = [];
    this.count = 0;
  }
  
  add(value) {
    this.count++;
    if (this.reservoir.length < this.capacity) {
      this.reservoir.push(value);
    } else {
      const j = Math.floor(Math.random() * this.count);
      if (j < this.capacity) {
        this.reservoir[j] = value;
      }
    }
  }

  getPercentile(p) {
    if (this.reservoir.length === 0) return 0;
    this.reservoir.sort((a, b) => a - b);
    return this.reservoir[Math.floor(this.reservoir.length * p)];
  }
}

// Data validation
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
const TIERS = (process.env.RPS_TIERS || '1000,5000,10000,16667,20000').split(',').map(Number);
const DURATION_SEC = parseInt(process.env.DURATION_SEC || "30");
const MAX_CONCURRENCY = parseInt(process.env.MAX_CONCURRENCY || "2000");

const client = new Motionmesh({
  apiKey: data.api_keys[0],
  baseURL: BASE_URL,
});

async function runTier(targetRPS) {
  console.log(`\n==============================================`);
  console.log(`Starting SDK Benchmark Tier: ${targetRPS} RPS`);
  console.log(`==============================================`);

  let completed = 0;
  let errors = 0;
  let dropped = 0;
  let inFlight = 0;
  const latencies = new ReservoirSampler(1024);
  
  const intervalMs = 1000 / targetRPS;
  const totalRequests = targetRPS * DURATION_SEC;
  let requestedCount = 0;

  const start = performance.now();
  const endPromise = new Promise(resolve => {
    const timer = setInterval(async () => {
      if (requestedCount >= totalRequests) {
        clearInterval(timer);
        return;
      }
      
      requestedCount++;
      
      if (inFlight >= MAX_CONCURRENCY) {
        dropped++;
        return;
      }

      inFlight++;
      const reqStart = performance.now();
      
      try {
        await client.videos.list({ limit: 10 });
        completed++;
        latencies.add(performance.now() - reqStart);
      } catch (err) {
        errors++;
      } finally {
        inFlight--;
        if (completed + errors + dropped >= totalRequests) {
          resolve();
        }
      }
    }, intervalMs);
  });

  await endPromise;
  
  const end = performance.now();
  const totalTimeSec = (end - start) / 1000;
  const actualRps = completed / totalTimeSec;
  
  const p50 = latencies.getPercentile(0.50);
  const p95 = latencies.getPercentile(0.95);
  const p99 = latencies.getPercentile(0.99);

  console.log(`Requested RPS:  ${targetRPS}`);
  console.log(`Actual RPS:     ${actualRps.toFixed(2)}`);
  console.log(`Total Requests: ${completed + errors + dropped}`);
  console.log(`Successful:     ${completed}`);
  console.log(`Failed:         ${errors}`);
  console.log(`Dropped:        ${dropped}`);
  console.log(`Time Elapsed:   ${totalTimeSec.toFixed(2)}s`);
  console.log(`Latency p50:    ${p50.toFixed(2)} ms`);
  console.log(`Latency p95:    ${p95.toFixed(2)} ms`);
  console.log(`Latency p99:    ${p99.toFixed(2)} ms`);
  
  if (dropped > (completed + errors) * 0.05) {
    console.log(`\n[WARNING] LOAD_GENERATOR_SATURATED - Node.js event loop could not keep up with MAX_CONCURRENCY (${MAX_CONCURRENCY})`);
  }

  const outDir = path.join(__dirname, '../../docs/benchmarks');
  if (!fs.existsSync(outDir)) {
    fs.mkdirSync(outDir, { recursive: true });
  }

  const report = {
    timestamp: new Date().toISOString(),
    benchmark_type: "official_sdk",
    target_rps: targetRPS,
    duration_s: totalTimeSec,
    requests: {
      requested: targetRPS * DURATION_SEC,
      success: completed,
      failed: errors,
      dropped: dropped
    },
    actual_rps: actualRps,
    latency_ms: {
      p50, p95, p99
    },
    system: {
      cpu_usage: process.cpuUsage(),
      memory_usage: process.memoryUsage()
    }
  };

  const outFile = path.join(outDir, `sdk-benchmark-${targetRPS}-${Date.now()}.json`);
  fs.writeFileSync(outFile, JSON.stringify(report, null, 2));
  console.log(`Artifact saved to: ${outFile}`);
}

async function runAll() {
  // Preflight
  try {
    await client.videos.list({ limit: 1 });
    console.log("Pre-flight OK.");
  } catch (err) {
    console.error("ABORT: Pre-flight failed.", err.message);
    process.exit(1);
  }

  for (const tier of TIERS) {
    await runTier(tier);
    // Cool down
    await new Promise(r => setTimeout(r, 5000));
  }
}

runAll().catch(console.error);
