const fs = require('fs');
const path = require('path');
const crypto = require('crypto');
const { Pool } = require('pg');
const { v4: uuidv4 } = require('uuid');

const NUM_ACCOUNTS = 100000;
const BATCH_SIZE = 1000;

async function generateData() {
  console.log(`Generating test data: ${NUM_ACCOUNTS} accounts in batches of ${BATCH_SIZE}...`);
  
  const pool = new Pool({
    connectionString: process.env.DATABASE_URL || 'postgres://postgres:postgres@localhost:5432/motionmesh?sslmode=disable',
    max: 10,
  });

  const apiKeys = [];
  const accountIds = [];
  const bucketIds = [];

  const client = await pool.connect();
  try {
    const planId = 'plan_benchmark';
    await client.query(`
      INSERT INTO plans (id, name, stripe_product_id, max_storage_gb, max_bandwidth_gb, price_cents)
      VALUES ($1, 'Benchmark Plan', 'prod_mock', 10000, 100000, 0)
      ON CONFLICT (id) DO NOTHING
    `, [planId]);

    let count = 0;
    while (count < NUM_ACCOUNTS) {
      await client.query('BEGIN');
      
      const batchAccounts = [];
      const batchApiKeys = [];
      
      const currentBatchSize = Math.min(BATCH_SIZE, NUM_ACCOUNTS - count);
      
      for (let i = 0; i < currentBatchSize; i++) {
        const accountId = uuidv4();
        
        // Generate a valid API key (mot_live_ + 16 hex chars . 64 hex chars)
        const prefixBytes = crypto.randomBytes(8);
        const secretBytes = crypto.randomBytes(32);
        const prefix = 'mot_live_' + prefixBytes.toString('hex');
        const secret = secretBytes.toString('hex');
        const rawKey = `${prefix}.${secret}`;
        const hash = crypto.createHash('sha256').update(secret).digest('hex');
        
        const bucketId = `bucket_${uuidv4()}`;
        
        accountIds.push(accountId);
        apiKeys.push(rawKey);
        bucketIds.push(bucketId);

        batchAccounts.push(`('${accountId}', '${planId}', 'cus_mock_${uuidv4().substring(0,8)}', NOW(), NOW())`);
        batchApiKeys.push(`('${hash}', '${accountId}', '${prefix}', NOW())`);
      }
      
      if (batchAccounts.length > 0) {
        await client.query(`
          INSERT INTO accounts (id, plan_id, stripe_customer_id, created_at, updated_at)
          VALUES ${batchAccounts.join(',')}
          ON CONFLICT (id) DO NOTHING
        `);
        
        await client.query(`
          INSERT INTO api_keys (hash, account_id, prefix, created_at)
          VALUES ${batchApiKeys.join(',')}
          ON CONFLICT DO NOTHING
        `);
      }
      
      await client.query('COMMIT');
      count += currentBatchSize;
      console.log(`Generated ${count} accounts...`);
    }

    const data = {
        account_ids: accountIds,
        api_keys: apiKeys,
        bucket_ids: bucketIds
    };

    const outPath = path.join(__dirname, 'data.json');
    fs.writeFileSync(outPath, JSON.stringify(data, null, 2), 'utf8');
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
