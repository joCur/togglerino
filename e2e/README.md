# Togglerino E2E Tests

End-to-end tests using [Playwright](https://playwright.dev/) covering the full Togglerino stack: React dashboard, Go management API, and SDK evaluation API.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (for the test environment)
- [Node.js](https://nodejs.org/) 20+

## Quick Start

```bash
cd e2e
npm install
npx playwright install chromium
npm test
```

This will:
1. Start PostgreSQL + Togglerino in Docker containers (if not already running)
2. Truncate the database for a clean state
3. Run all E2E tests
4. Tear down containers when done (if started by the test runner)

### Running Against a Local Dev Server

If you're already running Togglerino via `dev.sh`:

```bash
E2E_BASE_URL=http://localhost:8090 E2E_DATABASE_URL=postgres://togglerino:togglerino@localhost:5432/togglerino npm test
```

## Test Structure

```
tests/
├── setup/           # Playwright setup project (creates admin, saves session)
│   └── auth.setup   # Runs once before all smoke tests
├── smoke/           # Critical path tests (serial, ordered by filename)
│   ├── 01-setup     # Setup flow verification
│   ├── 02-auth      # Authentication flows
│   ├── 03-projects  # Project CRUD + default environments
│   ├── 04-flags     # Flag lifecycle
│   └── 05-logout    # Logout (runs last)
└── features/        # Feature-specific tests (parallel, isolated) — Phase 2+
```

## Writing Tests

Import fixtures from the helpers:

```typescript
import { test, expect } from '../../helpers/fixtures.js';
import { uniqueProjectKey } from '../../helpers/test-data.js';

test('my test', async ({ authenticatedPage: page, testProject, apiContext }) => {
  // authenticatedPage — pre-logged-in browser
  // testProject — isolated project (auto-created, auto-deleted)
  // apiContext — typed API client for setup/assertions
});
```

## Useful Commands

| Command | Description |
|---------|-------------|
| `npm test` | Run all tests |
| `npm run test:ui` | Playwright UI mode |
| `npm run test:headed` | Run with visible browser |
| `npm run test:debug` | Step-by-step debugging |

## CI

E2E tests run on every PR as the `test-e2e` job in GitHub Actions. On failure, Playwright HTML reports and traces are uploaded as artifacts (retained 7 days).

## Development Workflow

1. **Implement** the feature
2. **Plan** test scenarios (happy paths, edge cases)
3. **Explore** manually with Playwright MCP
4. **Automate** by writing spec files
