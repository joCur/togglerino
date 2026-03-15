import { execSync } from 'node:child_process';
import { writeFileSync, mkdirSync } from 'node:fs';
import path from 'node:path';
import pg from 'pg';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';
const DB_URL = process.env.E2E_DATABASE_URL || 'postgres://togglerino:togglerino@localhost:5433/togglerino?sslmode=disable';
const STATE_FILE = path.join(__dirname, 'test-results', '.e2e-state.json');

async function isServerRunning(): Promise<boolean> {
  try {
    const res = await fetch(`${BASE_URL}/healthz`);
    return res.ok;
  } catch {
    return false;
  }
}

async function waitForServer(timeoutMs = 120_000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (await isServerRunning()) return;
    await new Promise(r => setTimeout(r, 2_000));
  }
  throw new Error(`Server at ${BASE_URL} did not become ready within ${timeoutMs}ms`);
}

async function truncateDatabase(): Promise<void> {
  const client = new pg.Client({ connectionString: DB_URL });
  await client.connect();
  try {
    const { rows } = await client.query(`
      SELECT tablename FROM pg_tables
      WHERE schemaname = 'public'
        AND tablename NOT IN ('schema_migrations', 'roles')
    `);
    if (rows.length > 0) {
      const tables = rows.map(r => `"${r.tablename}"`).join(', ');
      await client.query(`TRUNCATE ${tables} CASCADE`);
    }
  } finally {
    await client.end();
  }
}

export default async function globalSetup() {
  let startedContainers = false;

  if (!(await isServerRunning())) {
    console.log('Server not running — starting Docker containers...');
    execSync(
      'COMPOSE_PROJECT_NAME=togglerino-e2e docker compose -f docker-compose.e2e.yml up -d --build',
      { cwd: __dirname, stdio: 'inherit' }
    );
    startedContainers = true;
    console.log('Waiting for server to be ready...');
    await waitForServer();
  } else {
    console.log(`Server already running at ${BASE_URL}`);
  }

  console.log('Truncating database for clean state...');
  await truncateDatabase();

  // Save state so global-teardown knows whether to stop containers
  mkdirSync(path.dirname(STATE_FILE), { recursive: true });
  writeFileSync(STATE_FILE, JSON.stringify({ startedContainers }));

  console.log('Global setup complete.');
}
