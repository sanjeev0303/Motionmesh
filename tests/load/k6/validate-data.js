const fs = require('fs');
const path = require('path');
const { Pool } = require('pg');

async function validateData() {
  const dataPath = path.join(__dirname, 'data.json');
  if (!fs.existsSync(dataPath)) {
    console.error(`Data file not found at ${dataPath}. Please run generate-data.js first.`);
    process.exit(1);
  }

  const data = JSON.parse(fs.readFileSync(dataPath, 'utf8'));
  const { account_ids, api_keys, bucket_ids } = data;

  console.log(`Validating data.json with ${account_ids.length} accounts...`);

  const pool = new Pool({
    connectionString: process.env.DATABASE_URL || 'postgres://postgres:postgres@localhost:5432/motionmesh?sslmode=disable'
  });

  const client = await pool.connect();
  try {
    const { rows: accountRows } = await client.query('SELECT COUNT(*) FROM accounts');
    const dbAccountCount = parseInt(accountRows[0].count, 10);
    console.log(`Database has ${dbAccountCount} total accounts.`);

    if (dbAccountCount < account_ids.length) {
      console.warn(`WARNING: Database account count (${dbAccountCount}) is less than data.json count (${account_ids.length}).`);
    }

    const { rows: keyRows } = await client.query('SELECT COUNT(*) FROM api_keys');
    const dbKeyCount = parseInt(keyRows[0].count, 10);
    console.log(`Database has ${dbKeyCount} total API keys.`);

    // Random sampling
    const sampleSize = Math.min(10, account_ids.length);
    console.log(`\nRandomly verifying ${sampleSize} samples...`);
    
    for (let i = 0; i < sampleSize; i++) {
      const idx = Math.floor(Math.random() * account_ids.length);
      const accId = account_ids[idx];
      const apiKey = api_keys[idx];
      const prefix = apiKey.split('.')[0];
      
      const { rows: checkRows } = await client.query('SELECT * FROM api_keys WHERE account_id = $1 AND prefix = $2', [accId, prefix]);
      
      if (checkRows.length === 1) {
        console.log(`[OK] Account ${accId} has valid key matching prefix ${prefix}`);
      } else {
        console.error(`[ERROR] Account ${accId} missing or mismatch for prefix ${prefix}`);
      }
    }
    
    console.log('\nValidation complete.');
  } catch (e) {
    console.error(e);
  } finally {
    client.release();
    await pool.end();
  }
}

validateData().catch(console.error);
