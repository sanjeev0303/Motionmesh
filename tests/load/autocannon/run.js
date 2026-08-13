const autocannon = require('autocannon');
const fs = require('fs');
const path = require('path');

const dataPath = path.join(__dirname, '../k6/data.json');
let apiKeys = [];

try {
  const data = JSON.parse(fs.readFileSync(dataPath, 'utf8'));
  apiKeys = data.api_keys;
} catch (err) {
  console.error('Error reading data.json (run generate-load-data first):', err);
  process.exit(1);
}

const url = process.env.BASE_URL || 'http://localhost:8080';

const instance = autocannon({
  url: url + '/api/v1/videos?limit=10',
  connections: 1000, // default
  pipelining: 1, // default
  duration: 30, // default
  requests: [
    {
      method: 'GET',
      path: '/api/v1/videos?limit=10',
      setupRequest: (req, context) => {
        const apiKey = apiKeys[Math.floor(Math.random() * apiKeys.length)];
        req.headers.Authorization = `Bearer ${apiKey}`;
        return req;
      }
    }
  ]
}, (err, result) => {
  if (err) {
    console.error('Error running autocannon:', err);
  }
});

autocannon.track(instance, { renderProgressBar: true });
