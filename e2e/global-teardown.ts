import { execSync } from 'node:child_process';
import { readFileSync, rmSync } from 'node:fs';
import path from 'node:path';

const STATE_FILE = path.join(__dirname, 'test-results', '.e2e-state.json');

export default async function globalTeardown() {
  try {
    const state = JSON.parse(readFileSync(STATE_FILE, 'utf-8'));
    if (state.startedContainers) {
      console.log('Stopping Docker containers...');
      execSync(
        'COMPOSE_PROJECT_NAME=togglerino-e2e docker compose -f docker-compose.e2e.yml down -v',
        { cwd: __dirname, stdio: 'inherit' }
      );
    }
    rmSync(STATE_FILE, { force: true });
  } catch {
    // State file missing — nothing to clean up
  }
}
