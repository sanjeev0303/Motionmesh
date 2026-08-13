const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { Pool } = require('pg');
const { v4: uuidv4 } = require('uuid');

// Increased to 100k to ensure we don't just benchmark PostgreSQL cache hits
const NUM_ACCOUNTS = 100000;
const API_URL = process.env.API_URL || 'http://localhost:8080';

async function generateData() {
  console.log(`Generating test data: ${NUM_ACCOUNTS} accounts...`);
  console.log(`This will take a significant amount of time and create realistic database entropy.`);
  
  const pool = new Pool({
    connectionString: process.env.DATABASE_URL || 'postgres://postgres:postgres@localhost:5432/motionmesh?sslmode=disable'
  });

  const apiKeys = [];
  const accountIds = [];
  const bucketIds = [];

  const client = await pool.connect();
  try {
    await client.query('BEGIN');
    
    // Create benchmark plan
    const planId = 'plan_benchmark';
    await client.query(`
      INSERT INTO plans (id, name, stripe_product_id, max_storage_gb, max_bandwidth_gb, price_cents)
      VALUES ($1, 'Benchmark Plan', 'prod_mock', 10000, 100000, 0)
      ON CONFLICT (id) DO NOTHING
    `, [planId]);

    let count = 0;
    
    for (let i = 0; i < NUM_ACCOUNTS; i++) {
      const accountId = `acc_bench_${uuidv4().substring(0, 8)}`;
      const apiKey = `mot_bench_${crypto.randomBytes(16).toString('hex')}`;
      const bucketId = `bucket_${uuidv4()}`;
      
      accountIds.push(accountId);
      apiKeys.push(apiKey);
      bucketIds.push(bucketId);

      await client.query(`
        INSERT INTO accounts (id, plan_id, stripe_customer_id, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
      `, [accountId, planId, `cus_mock_${uuidv4().substring(0,8)}`]);

      await client.query(`
        INSERT INTO api_keys (key_hash, account_id, prefix, created_at)
        VALUES ($1, $2, $3, NOW())
      `, [crypto.createHash('sha256').update(apiKey).digest('hex'), accountId, apiKey.substring(0, 14)]);

      if (i % 1000 === 0 && i > 0) {
          console.log(`Generated ${i} accounts...`);
      }
    }

    const data = {
        account_ids: accountIds,
        api_keys: apiKeys,
        bucket_ids: bucketIds
    };

    const outPath = path.join(__dirname, 'data.json');
    fs.writeFileSync(outPath, JSON.stringify(data, null, 2), 'utf8');
    
    await client.query('COMMIT');
    console.log(`Wrote dataset to ${outPath}`);
  } catch (e) {
    await client.query('ROLLBACK');
    throw e;
  } finally {
    client.release();
    await pool.end();
  }
}

generateData().catch(console.error);
