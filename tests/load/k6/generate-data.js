#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const NUM_KEYS = 100000;
const NUM_VIDEOS = 1000000;

function generateData() {
  console.log(`Generating dataset mapping for ${NUM_KEYS} accounts and ${NUM_VIDEOS} videos...`);
  
  const data = {
    api_keys: [],
    video_ids: []
  };

  // Generate synthetic keys (in a real scenario, this might pull from DB or copy from DB dump)
  for (let i = 0; i < NUM_KEYS; i++) {
    data.api_keys.push(`sk_test_${Math.random().toString(36).substring(2, 15)}`);
  }

  // Generate synthetic video IDs
  for (let i = 0; i < Math.min(NUM_VIDEOS, 50000); i++) { // cap at 50k for JSON size
    data.video_ids.push(`vid_${Math.random().toString(36).substring(2, 10)}`);
  }

  const outPath = path.join(__dirname, 'data.json');
  fs.writeFileSync(outPath, JSON.stringify(data, null, 2), 'utf8');
  console.log(`Wrote dataset to ${outPath}`);
}

generateData();
