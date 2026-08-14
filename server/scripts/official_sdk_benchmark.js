const { MotionMeshClient } = require('@motionmesh/sdk');
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

let BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
if (process.env.AWS_MODE === 'true') {
  BASE_URL = 'https://api.motionmesh.co.in/v1';
}

const CLIENT_TYPE = process.env.CLIENT_TYPE || 'sdk';
const TIERS = (process.env.RPS_TIERS || '1000,5000,10000,16667,20000').split(',').map(Number);
const DURATION_SEC = parseInt(process.env.DURATION_SEC || "30");
const MAX_CONCURRENCY = parseInt(process.env.MAX_CONCURRENCY || "2000");

const client = new MotionMeshClient({
  apiKey: data.api_keys[0],
  baseURL: BASE_URL,
});

const httpHeaders = { 'Authorization': `Bearer ${data.api_keys[0]}` };

async function runTier(targetRPS) {
  console.log(`\n==============================================`);
  console.log(`Starting SDK Benchmark Tier: ${targetRPS} RPS`);
  console.log(`==============================================`);

  let successful = 0;
  let failed = 0;
  let dropped = 0;
  let sent = 0;
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
      sent++;
      const reqStart = performance.now();
      
      try {
        const op = Math.random();
        
        if (CLIENT_TYPE === 'sdk') {
            let p;
            if (op < 0.4) p = client.videos.list({ limit: 10 });
            else if (op < 0.6) p = client.buckets.list();
            else if (op < 0.8 && data.video_ids && data.video_ids.length > 0) p = client.videos.get(data.video_ids[Math.floor(Math.random() * data.video_ids.length)]);
            else if (op < 0.9 && data.video_ids && data.video_ids.length > 0) p = client.videos.playback(data.video_ids[Math.floor(Math.random() * data.video_ids.length)]);
            else p = client.mediaConverter.listJobs({ limit: 10 });
            await p;
        } else {
            let url;
            if (op < 0.4) url = `${BASE_URL}/videos?limit=10`;
            else if (op < 0.6) url = `${BASE_URL}/buckets`;
            else if (op < 0.8 && data.video_ids && data.video_ids.length > 0) url = `${BASE_URL}/videos/${data.video_ids[Math.floor(Math.random() * data.video_ids.length)]}`;
            else if (op < 0.9 && data.video_ids && data.video_ids.length > 0) url = `${BASE_URL}/videos/${data.video_ids[Math.floor(Math.random() * data.video_ids.length)]}/playback`;
            else url = `${BASE_URL}/jobs?limit=10`;
            
            const res = await fetch(url, { headers: httpHeaders });
            if (!res.ok) throw new Error("HTTP error " + res.status);
            if (res.status !== 204) await res.json();
        }
        
        successful++;
        latencies.add(performance.now() - reqStart);
      } catch (err) {
        failed++;
      } finally {
        inFlight--;
        if (successful + failed + dropped >= totalRequests) {
          resolve();
        }
      }
    }, intervalMs);
  });

  await endPromise;
  
  const end = performance.now();
  const totalTimeSec = (end - start) / 1000;
  const actualRps = successful / totalTimeSec;
  
  const p50 = latencies.getPercentile(0.50);
  const p95 = latencies.getPercentile(0.95);
  const p99 = latencies.getPercentile(0.99);

  console.log(`Client Type:    ${CLIENT_TYPE}`);
  console.log(`Requested RPS:  ${targetRPS}`);
  console.log(`Actual RPS:     ${actualRps.toFixed(2)}`);
  console.log(`Total Requests: ${requestedCount}`);
  console.log(`Sent:           ${sent}`);
  console.log(`Completed:      ${successful + failed}`);
  console.log(`Successful:     ${successful}`);
  console.log(`Failed:         ${failed}`);
  console.log(`Dropped:        ${dropped}`);
  console.log(`Time Elapsed:   ${totalTimeSec.toFixed(2)}s`);
  console.log(`Latency p50:    ${p50.toFixed(2)} ms`);
  console.log(`Latency p95:    ${p95.toFixed(2)} ms`);
  console.log(`Latency p99:    ${p99.toFixed(2)} ms`);
  
  if (dropped > (successful + failed) * 0.05) {
    console.log(`\n[WARNING] LOAD_GENERATOR_SATURATED - Node.js event loop could not keep up with MAX_CONCURRENCY (${MAX_CONCURRENCY})`);
  }

  const outDir = path.join(__dirname, '../../docs/benchmarks');
  if (!fs.existsSync(outDir)) {
    fs.mkdirSync(outDir, { recursive: true });
  }

  const report = {
    timestamp: new Date().toISOString(),
    benchmark_type: "official_sdk",
    client_type: CLIENT_TYPE,
    target_rps: targetRPS,
    duration_s: totalTimeSec,
    requests: {
      requested: requestedCount,
      sent: sent,
      completed: successful + failed,
      success: successful,
      failed: failed,
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
