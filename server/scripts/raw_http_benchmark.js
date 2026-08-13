/**
 * Motionmesh SDK Benchmarking Tool
 * 
 * Simulates high-throughput SDK usage with connection pooling
 * to test 1M RPM (16,667 RPS) headroom and connection reuse.
 */

const http = require('http');
const https = require('https');
const { performance } = require('perf_hooks');

const TARGET_RPS = parseInt(process.env.TARGET_RPS) || 20000;
const DURATION_SEC = parseInt(process.env.DURATION_SEC) || 10;
const API_URL = process.env.API_URL || 'http://localhost:8080/api/v1/health';
const CONCURRENCY = parseInt(process.env.CONCURRENCY) || 500;

// Mock SDK Implementation with Keep-Alive Agent
class MotionmeshClient {
  constructor(baseURL) {
    this.baseURL = new URL(baseURL);
    this.isHttps = this.baseURL.protocol === 'https:';
    
    // Crucial for SDK performance: Connection pooling
    const agentOpts = {
      keepAlive: true,
      maxSockets: CONCURRENCY, 
      maxFreeSockets: CONCURRENCY,
      timeout: 5000,
    };
    
    this.agent = this.isHttps ? new https.Agent(agentOpts) : new http.Agent(agentOpts);
  }

  async ping() {
    return new Promise((resolve, reject) => {
      const req = (this.isHttps ? https : http).request(this.baseURL, {
        method: 'GET',
        agent: this.agent,
        headers: {
          'User-Agent': 'motionmesh-node-sdk/1.0',
          'Authorization': 'Bearer benchmark-token'
        }
      }, (res) => {
        // Consume response data to free up the socket
        res.on('data', () => {});
        res.on('end', () => {
          if (res.statusCode >= 200 && res.statusCode < 300) {
            resolve(res.statusCode);
          } else {
            reject(new Error(`Status: ${res.statusCode}`));
          }
        });
      });

      req.on('error', reject);
      req.end();
    });
  }
}

async function runBenchmark() {
  console.log(`Starting SDK Benchmark -> ${API_URL}`);
  console.log(`Target: ${TARGET_RPS} RPS for ${DURATION_SEC} seconds`);
  console.log(`Concurrency Limit: ${CONCURRENCY}`);
  console.log(`\nWarming up connections...`);

  const client = new MotionmeshClient(API_URL);

  let successCount = 0;
  let errorCount = 0;
  let startTime = performance.now();
  
  // Rate Limiting Logic
  const delay = (ms) => new Promise(res => setTimeout(res, ms));

  // Run a constant flow
  const endTime = startTime + (DURATION_SEC * 1000);
  
  // Create a pool of workers
  const workers = Array.from({ length: CONCURRENCY }).map(async () => {
    while (performance.now() < endTime) {
      try {
        await client.ping();
        successCount++;
      } catch (err) {
        errorCount++;
      }
    }
  });

  await Promise.all(workers);

  const totalTimeSec = (performance.now() - startTime) / 1000;
  const actualRps = (successCount + errorCount) / totalTimeSec;

  console.log(`\n--- BENCHMARK RESULTS ---`);
  console.log(`Total Requests:    ${successCount + errorCount}`);
  console.log(`Successful:        ${successCount}`);
  console.log(`Failed:            ${errorCount}`);
  console.log(`Time Elapsed:      ${totalTimeSec.toFixed(2)}s`);
  console.log(`Actual RPS:        ${actualRps.toFixed(2)} req/sec`);
  
  if (actualRps >= TARGET_RPS * 0.9) {
    console.log(`\n✅ Headroom target achieved!`);
  } else {
    console.log(`\n⚠️ Headroom target missed. Tuned infra needed.`);
  }

  process.exit(0);
}

runBenchmark().catch(console.error);
