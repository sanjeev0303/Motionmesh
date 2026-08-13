#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const BENCHMARKS_DIR = path.join(__dirname, '../docs/benchmarks/results');
const REPORT_FILE = path.join(__dirname, '../docs/investor/scalability-report.md');

function readBenchmarkData() {
  if (!fs.existsSync(BENCHMARKS_DIR)) {
    return null;
  }
  
  // Here we would parse JSON files outputted by k6 / autocannon
  // For now, this is a scaffold that looks for latest results
  const result = {
    rpm: 'NOT YET MEASURED',
    vus: 'NOT YET MEASURED',
    billingLatency: 'NOT YET MEASURED',
    transcodeLatency: 'NOT YET MEASURED',
    timestamp: new Date().toISOString()
  };

  const mixedProdFile = path.join(BENCHMARKS_DIR, 'production-mixed-result.json');
  if (fs.existsSync(mixedProdFile)) {
    const data = JSON.parse(fs.readFileSync(mixedProdFile, 'utf8'));
    // K6 output mapping
    const rps = data.metrics?.http_reqs?.values?.rate || 0;
    if (rps > 0) {
      result.rpm = `${Math.round(rps * 60).toLocaleString()} RPM`;
    }
    const vus = data.metrics?.vus_max?.values?.value || 0;
    if (vus > 0) {
      result.vus = `${vus.toLocaleString()}`;
    }
  }

  return result;
}

function updateReport(data) {
  if (!data) return;

  let report = fs.readFileSync(REPORT_FILE, 'utf8');

  // Replace placeholder metrics with actuals
  report = report.replace(/\| Requests Per Minute \| (.*) \| (.*) \|/g, `| Requests Per Minute | ${data.rpm} | NOT YET MEASURED |`);
  report = report.replace(/\| Active VUs \/ Connections \| (.*) \| (.*) \|/g, `| Active VUs / Connections | ${data.vus} | NOT YET MEASURED |`);
  report = report.replace(/\| Stripe Billing Event Latency \| (.*) \| (.*) \|/g, `| Stripe Billing Event Latency | ${data.billingLatency} | NOT YET MEASURED |`);
  report = report.replace(/\| Transcode Dispatch Latency \| (.*) \| (.*) \|/g, `| Transcode Dispatch Latency | ${data.transcodeLatency} | NOT YET MEASURED |`);

  fs.writeFileSync(REPORT_FILE, report, 'utf8');
  console.log('Successfully updated scalability report with real benchmark data.');
}

const data = readBenchmarkData();
updateReport(data);
