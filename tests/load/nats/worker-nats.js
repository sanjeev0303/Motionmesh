const { connect, StringCodec } = require('nats');
const fs = require('fs');

const RPS_TARGET = parseInt(process.env.RPS || '1000', 10);
const DURATION_SECONDS = parseInt(process.env.DURATION || '60', 10);
const MAX_CONCURRENCY = parseInt(process.env.MAX_CONCURRENCY || '2000', 10);
const NATS_URL = process.env.NATS_URL || 'nats://localhost:4222';

let data = { video_ids: ['test-video'], account_ids: ['test-account'] };
try {
  data = JSON.parse(fs.readFileSync('./tests/load/k6/data.json', 'utf-8'));
} catch (e) {
  console.warn('Could not read data.json, using fallback data.');
}

async function run() {
  const nc = await connect({ servers: NATS_URL });
  const js = nc.jetstream();
  const sc = StringCodec();
  
  let successCount = 0;
  let errorCount = 0;
  let droppedCount = 0;
  let currentConcurrency = 0;
  let latencies = [];
  
  console.log(`Starting NATS benchmark: ${RPS_TARGET} ops/sec for ${DURATION_SECONDS}s`);
  
  const intervalMs = 1000 / RPS_TARGET;
  const totalRequests = RPS_TARGET * DURATION_SECONDS;
  let requestedRPSCount = 0;

  const intervalId = setInterval(async () => {
    if (requestedRPSCount >= totalRequests) {
      clearInterval(intervalId);
      return;
    }
    
    if (currentConcurrency >= MAX_CONCURRENCY) {
      droppedCount++;
      requestedRPSCount++;
      return;
    }

    currentConcurrency++;
    requestedRPSCount++;
    const start = process.hrtime.bigint();
    
    const videoId = data.video_ids[requestedRPSCount % data.video_ids.length];
    const accountId = data.account_ids ? data.account_ids[requestedRPSCount % data.account_ids.length] : 'acc-123';
    
    const payload = JSON.stringify({
      id: `job-${Date.now()}-${Math.random()}`,
      video_id: videoId,
      account_id: accountId,
      type: 'transcode',
      mock_ffmpeg: process.env.MOCK_FFMPEG !== 'false'
    });

    try {
      await js.publish('jobs.transcode.init', sc.encode(payload));
      successCount++;
      const end = process.hrtime.bigint();
      const latencyMs = Number(end - start) / 1_000_000;
      latencies.push(latencyMs);
    } catch (err) {
      errorCount++;
    } finally {
      currentConcurrency--;
    }
  }, intervalMs);

  await new Promise(resolve => setTimeout(resolve, DURATION_SECONDS * 1000 + 2000));
  
  await nc.close();
  
  console.log(`\n--- NATS Worker Benchmark Results ---`);
  console.log(`Requested Ops/sec: ${Math.round(requestedRPSCount / DURATION_SECONDS)}`);
  console.log(`Actual Ops/sec:    ${Math.round(successCount / DURATION_SECONDS)}`);
  console.log(`Success:           ${successCount}`);
  console.log(`Errors:            ${errorCount}`);
  console.log(`Dropped:           ${droppedCount}`);
  
  if (droppedCount > successCount * 0.1) {
    console.log(`\n[WARNING] LOAD_GENERATOR_SATURATED - NATS client fell behind or was throttled by MAX_CONCURRENCY.`);
  }

  if (latencies.length > 0) {
    latencies.sort((a, b) => a - b);
    const avg = latencies.reduce((a, b) => a + b, 0) / latencies.length;
    const p50 = latencies[Math.floor(latencies.length * 0.50)];
    const p95 = latencies[Math.floor(latencies.length * 0.95)];
    const p99 = latencies[Math.floor(latencies.length * 0.99)];
    console.log(`Avg Publish Latency: ${avg.toFixed(2)}ms`);
    console.log(`p50 Publish Latency: ${p50.toFixed(2)}ms`);
    console.log(`p95 Publish Latency: ${p95.toFixed(2)}ms`);
    console.log(`p99 Publish Latency: ${p99.toFixed(2)}ms`);
  }
}

run().catch(console.error);
