const { MotionMesh } = require('@motionmesh/sdk/server');
const fs = require('fs');
const path = require('path');
const { Worker, isMainThread, parentPort, workerData } = require('worker_threads');

const TARGET_RPM = 1000000;
const TARGET_RPS = Math.ceil(TARGET_RPM / 60); // 16667
const DURATION_SEC = parseInt(process.env.DURATION) || 60;
const NUM_WORKERS = parseInt(process.env.NUM_WORKERS) || 8;
const RPS_PER_WORKER = Math.ceil(TARGET_RPS / NUM_WORKERS);

const dataPath = path.join(__dirname, 'k6/data.json');
let apiKeys = [];
let videoIds = [];

try {
  const data = JSON.parse(fs.readFileSync(dataPath, 'utf8'));
  apiKeys = data.api_keys;
  videoIds = data.video_ids;
} catch (err) {
  console.error('Error reading data.json. Run generate-data.js first.');
  process.exit(1);
}

if (isMainThread) {
  console.log(`Starting Node.js SDK 1M RPM load test`);
  console.log(`Target: ${TARGET_RPS} RPS across ${NUM_WORKERS} workers for ${DURATION_SEC}s`);

  let totalRequests = 0;
  let totalSuccess = 0;
  let workersCompleted = 0;

  for (let i = 0; i < NUM_WORKERS; i++) {
    const worker = new Worker(__filename, {
      workerData: {
        rps: RPS_PER_WORKER,
        duration: DURATION_SEC,
        apiKeys,
        videoIds,
      }
    });

    worker.on('message', (msg) => {
      totalRequests += msg.requests;
      totalSuccess += msg.success;
    });

    worker.on('exit', () => {
      workersCompleted++;
      if (workersCompleted === NUM_WORKERS) {
        console.log('Load test completed.');
        console.log(`Total Requests: ${totalRequests}`);
        console.log(`Successful Requests: ${totalSuccess}`);
        console.log(`Actual RPS: ${(totalRequests / DURATION_SEC).toFixed(2)}`);
      }
    });
  }
} else {
  const { rps, duration, apiKeys, videoIds } = workerData;
  const baseUrl = process.env.BASE_URL || 'http://localhost:8080';
  
  // Pre-initialize SDK instances for each key to avoid overhead
  const sdks = apiKeys.map(key => new MotionMesh({ apiKey: key, baseURL: baseUrl }));
  
  let reqs = 0;
  let success = 0;
  
  const interval = setInterval(async () => {
    // Fire 'rps' requests in parallel every second
    const promises = [];
    for (let i = 0; i < rps; i++) {
      const sdk = sdks[Math.floor(Math.random() * sdks.length)];
      const rand = Math.random();
      let p;
      if (rand < 0.5) {
        p = sdk.videos.list({ limit: 10 }).then(() => success++).catch(() => {});
      } else {
        const vid = videoIds[Math.floor(Math.random() * videoIds.length)];
        p = sdk.videos.get(vid).then(() => success++).catch(() => {});
      }
      promises.push(p);
      reqs++;
    }
    // We don't await here to let the next interval fire exactly on time
  }, 1000);

  setTimeout(() => {
    clearInterval(interval);
    // Give some time for pending requests to finish
    setTimeout(() => {
      parentPort.postMessage({ requests: reqs, success });
      process.exit(0);
    }, 2000);
  }, duration * 1000);
}
