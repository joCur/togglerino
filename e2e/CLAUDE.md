# E2E Tests

Playwright-based end-to-end tests for Togglerino. Tests run against the full stack: React UI + Go backend + PostgreSQL.

## Commands

```bash
npm test                    # Run all E2E tests (starts Docker containers if needed)
npm run test:ui             # Playwright UI mode (visual test runner)
npm run test:headed         # Run tests in visible browser
npm run test:debug          # Debug mode with Playwright Inspector
```

## Running Against Local Dev Server

Set `E2E_BASE_URL` to point at your dev server (from `dev.sh`) and `E2E_DATABASE_URL` to point at its Postgres:

```bash
E2E_BASE_URL=http://localhost:8090 E2E_DATABASE_URL=postgres://togglerino:togglerino@localhost:5432/togglerino npm test
```

Without these env vars, tests start their own Docker containers (port 9091, DB port 5433).

## Test Structure

- `tests/setup/auth.setup.ts` — Playwright setup project: creates admin user, saves authenticated session to `test-results/.auth/user.json`. Runs once before all smoke tests.
- `tests/smoke/` — Critical path smoketests, run serially (numeric prefix controls order). Each test gets the pre-authenticated session via `storageState`.
  - `01-setup.spec.ts` — Verifies setup is complete, rejects duplicate setup
  - `02-auth.spec.ts` — Session validity, invalid password, session persistence, unauthenticated redirect
  - `03-projects.spec.ts` — Project CRUD + default environments
  - `04-flags.spec.ts` — Flag lifecycle (create, toggle on/off, persist, archive, value types)
  - `05-logout.spec.ts` — Logout flow (runs last, uses isolated session to avoid invalidating shared state)
- `tests/features/` — Feature tests (Phase 2+, run in parallel with `testProject` fixture for isolation)

## Helpers

- `helpers/fixtures.ts` — Custom Playwright fixtures:
  - `authenticatedPage` — Page with admin session pre-loaded via storageState
  - `apiContext` — Typed `ApiHelper` instance with admin session for API calls (shared across tests to avoid rate limiting)
  - `testProject` — Creates/cleans up an isolated project per test
- `helpers/api.ts` — Typed API client wrapping Playwright's `request` context. Methods: `createProject()`, `createFlag()`, `updateFlagEnvConfig()`, `archiveFlag()`, `getFlag()`, etc.
- `helpers/auth.ts` — `ensureSetup()` (idempotent admin creation), `login()` (API-based), `logout()` (UI-based)
- `helpers/test-data.ts` — `uniqueProjectKey()`, `uniqueFlagKey()`, `uniqueEmail()`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`

## Writing New Tests

1. Import `{ test, expect }` from `../../helpers/fixtures.js` (NOT from `@playwright/test`) to get custom fixtures
2. Use `testProject` fixture for test isolation — each test gets its own project
3. Use `apiContext` for API-level setup (creating test data) and `authenticatedPage` for UI assertions
4. Put critical path tests in `smoke/` with numeric prefix, feature tests in `features/`
5. Smoke tests share a session (serial). Feature tests must be independent (parallel-safe)

## Rate Limiting

Auth endpoints are rate-limited at 10 req/60s per IP. The test infrastructure handles this by:
- Using a Playwright setup project that logs in once and saves `storageState`
- Sharing a single `apiContext` session across all tests in a worker
- Using API login (`POST /api/v1/auth/login`) instead of UI login where possible
- Tests that need fresh/unauthenticated contexts create them with `browser.newContext({ storageState: { cookies: [], origins: [] } })`

## Development Workflow

When adding E2E tests for a new feature:

1. **Implement the feature** (backend + frontend)
2. **Plan test scenarios** — list happy paths, edge cases, integration points
3. **Explore manually with Playwright MCP** — use Claude + Playwright MCP tools to interactively test the UI and API
4. **Write automated tests** — crystallize manual explorations into spec files using the helpers above

## Debugging

- **Traces**: `test-results/` contains traces for failed tests. Open with `npx playwright show-trace <path>`
- **Headed mode**: `npm run test:headed` shows the browser while tests run
- **Debug mode**: `npm run test:debug` opens Playwright Inspector for step-by-step debugging
- **Screenshots**: Failed tests auto-capture screenshots in `test-results/`
