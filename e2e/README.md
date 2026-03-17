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
3. Run all E2E tests (~89 tests in ~1 minute)
4. Tear down containers when done

### Running Against a Local Dev Server

```bash
E2E_BASE_URL=http://localhost:8090 \
E2E_DATABASE_URL=postgres://togglerino:togglerino@localhost:5432/togglerino \
npm test
```

> **Tip:** The E2E Docker environment disables auth rate limiting (`RATE_LIMIT_DISABLED=true`). Set this on your dev server too if running permission tests locally.

## Test Structure

```
tests/
├── setup/           # Playwright setup project (creates admin, saves session)
│   └── auth.setup
├── smoke/           # Critical path tests (serial, ordered by filename)
│   ├── 01-setup     # Setup flow verification
│   ├── 02-auth      # Authentication flows
│   ├── 03-projects  # Project CRUD + default environments
│   ├── 04-flags     # Flag lifecycle
│   └── 05-logout    # Logout (runs last)
└── features/        # Feature tests
    ├── boolean-flags        # Boolean flag evaluation behavior
    ├── sdk-evaluation       # SDK key + flag evaluation API
    ├── targeting-rules      # Condition operators, rollouts
    ├── segments / segments-ui  # Segment CRUD + segment_match
    ├── sse-streaming        # Real-time notifications
    ├── flag-management      # Full flag management UI
    ├── permissions          # Invite, accept, role restrictions
    ├── custom-roles         # Role CRUD + enforcement
    ├── environments         # Environment management
    ├── sdk-keys-ui          # SDK key generation + revocation
    ├── unknown-flags        # Unknown flag detection + dismissal
    ├── flag-templates       # Template CRUD + usage
    ├── lifecycle            # Lifecycle board
    ├── kill-switches        # Kill switch dashboard
    ├── playground           # Evaluation playground
    ├── personal-access-tokens  # PAT create + revoke
    └── account              # Profile + password management
```

## Writing Tests

```typescript
import { test, expect } from '../../helpers/fixtures.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

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
