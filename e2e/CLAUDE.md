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

Set `E2E_BASE_URL` and `E2E_DATABASE_URL` to point at your dev server (from `dev.sh`):

```bash
E2E_BASE_URL=http://localhost:8090 E2E_DATABASE_URL=postgres://togglerino:togglerino@localhost:5432/togglerino npm test
```

Without these env vars, tests start their own Docker containers (port 9091, DB port 5433). The E2E Docker environment sets `RATE_LIMIT_DISABLED=true` to avoid auth rate limiting during tests. When running against a local dev server, you may want to set this env var on the server too.

## Test Structure

Tests use Playwright's setup project pattern for authentication:

- `tests/setup/auth.setup.ts` — Runs once before all tests: creates admin user, saves session to `test-results/.auth/user.json`
- `tests/smoke/` — Critical path smoketests (serial, numeric prefix controls order). Use `storageState` from setup.
  - `01-setup.spec.ts` — Verifies setup complete, rejects duplicate
  - `02-auth.spec.ts` — Session, invalid password, persistence, unauthenticated redirect
  - `03-projects.spec.ts` — Project CRUD + default environments
  - `04-flags.spec.ts` — Flag create, toggle, persist, archive, value types
  - `05-logout.spec.ts` — Logout (runs last, uses isolated session)
- `tests/features/` — Feature tests covering specific functionality areas:
  - `boolean-flags.spec.ts` — Boolean evaluation: enabled=true, disabled=false, targeting rules
  - `sdk-evaluation.spec.ts` — SDK key creation, flag evaluation, value types, env scoping
  - `targeting-rules.spec.ts` — Condition operators, rule ordering, AND logic, rollouts
  - `segments.spec.ts` / `segments-ui.spec.ts` — Segment CRUD and segment_match evaluation
  - `sse-streaming.spec.ts` — SSE notifications trigger re-evaluation
  - `flag-management.spec.ts` — Delete, owner, variants, targeting rule builder, promote, lock, search
  - `permissions.spec.ts` — Invite/accept, member restrictions, viewer/project admin roles
  - `custom-roles.spec.ts` — Role CRUD, permission enforcement in API and UI
  - `environments.spec.ts` — Create, delete, navigate to SDK keys
  - `sdk-keys-ui.spec.ts` — Generate, revoke, per-environment isolation
  - `unknown-flags.spec.ts` — Non-existent flag evaluation → appears in UI, dismiss
  - `flag-templates.spec.ts` — Create template, use in flag creation, delete
  - `lifecycle.spec.ts` — Lifecycle board summary cards, API summary
  - `kill-switches.spec.ts` — Empty state, matrix display, toggle with confirmation
  - `playground.spec.ts` — Evaluate flags with context attributes
  - `personal-access-tokens.spec.ts` — Create PAT and use for API access, revoke
  - `account.spec.ts` — Display name, password change, account info

## Helpers

- `helpers/fixtures.ts` — Custom Playwright fixtures:
  - `authenticatedPage` — Page with admin session pre-loaded via `storageState`
  - `apiContext` — Shared `ApiHelper` instance (single login, reused across tests)
  - `testProject` — Creates/cleans up an isolated project per test
- `helpers/api.ts` — Typed API client + `SDKClient` for evaluation (Bearer token auth)
- `helpers/auth.ts` — `ensureSetup()`, `login()` (API-based), `logout()` (UI-based)
- `helpers/test-data.ts` — Unique name factories, admin credentials

## Writing New Tests

1. Import `{ test, expect }` from `../../helpers/fixtures.js` to get custom fixtures
2. Use `testProject` fixture for isolation — each test gets its own project
3. Use `apiContext` for API setup and `authenticatedPage` for UI assertions
4. For boolean flags: enabled=true, disabled=false, targeting rules use `"true"`/`"false"` variants
5. For tests needing distinct matched/unmatched values, use string flags with variants
6. Tests creating multiple users should handle auth rate limiting (10 req/60s per IP)

## Development Workflow

1. **Implement the feature** (backend + frontend)
2. **Plan test scenarios** — happy paths, edge cases, integration points
3. **Explore manually with Playwright MCP** — interactive testing with Claude
4. **Write automated tests** — crystallize into spec files using helpers

## Debugging

- **Traces**: `npx playwright show-trace test-results/<test>/trace.zip`
- **Headed mode**: `npm run test:headed`
- **Debug mode**: `npm run test:debug` (Playwright Inspector)
- **Screenshots**: Auto-captured on failure in `test-results/`
