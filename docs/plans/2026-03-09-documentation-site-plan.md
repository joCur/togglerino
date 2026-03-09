# Documentation Site Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a comprehensive Docusaurus 3 documentation site in `docs-site/` with full content across 6 sections, themed with Togglerino branding, deployed via GitHub Pages on release.

**Architecture:** Docusaurus 3 with TypeScript config, flat docs structure (single sidebar, 6 categories), classic theme with amber accent + Sora/Fira Code fonts. Docs serve as the site landing page (`routeBasePath: '/'`). 30 Markdown content pages covering Getting Started, Self-Hosting, Core Concepts, Dashboard Guide, SDKs, and API Reference.

**Tech Stack:** Docusaurus 3, React 19, TypeScript, GitHub Actions (deploy-pages), GitHub Pages

**Design doc:** `docs/plans/2026-03-09-documentation-site-design.md`

---

### Task 1: Docusaurus Scaffolding, Config, and Theme

**Files:**
- Create: `docs-site/package.json`
- Create: `docs-site/tsconfig.json`
- Create: `docs-site/docusaurus.config.ts`
- Create: `docs-site/sidebars.ts`
- Create: `docs-site/src/css/custom.css`
- Create: `docs-site/static/img/.gitkeep`
- Create: `docs-site/docs/getting-started/introduction.md` (placeholder — will be replaced in Task 2)

**Step 1: Scaffold Docusaurus project**

Run from repo root:

```bash
cd docs-site
npm init -y
npm install @docusaurus/core @docusaurus/preset-classic react react-dom
npm install --save-dev @docusaurus/module-type-aliases @docusaurus/types typescript
```

**Step 2: Create `docs-site/tsconfig.json`**

```json
{
  "extends": "@docusaurus/module-type-aliases/tsconfig.json",
  "compilerOptions": {
    "baseUrl": "."
  }
}
```

**Step 3: Create `docs-site/docusaurus.config.ts`**

```typescript
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Togglerino',
  tagline: 'Self-hosted feature flag management',
  favicon: 'img/favicon.ico',

  url: 'https://togglerino.github.io',
  baseUrl: '/togglerino/',

  organizationName: 'togglerino',
  projectName: 'togglerino',

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    navbar: {
      title: 'Togglerino',
      items: [
        {
          href: 'https://github.com/togglerino/togglerino',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            { label: 'Getting Started', to: '/' },
            { label: 'Self-Hosting', to: '/self-hosting/installation' },
            { label: 'SDKs', to: '/sdks/overview' },
          ],
        },
        {
          title: 'More',
          items: [
            { label: 'GitHub', href: 'https://github.com/togglerino/togglerino' },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Togglerino`,
    },
    colorMode: {
      defaultMode: 'light',
      respectPrefersColorScheme: true,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
```

**Step 4: Create `docs-site/sidebars.ts`**

```typescript
import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'getting-started/introduction',
        'getting-started/quick-start',
        'getting-started/first-flag-in-code',
      ],
      collapsed: false,
    },
    {
      type: 'category',
      label: 'Self-Hosting',
      items: [
        'self-hosting/installation',
        'self-hosting/configuration',
        'self-hosting/production',
        'self-hosting/upgrading',
      ],
    },
    {
      type: 'category',
      label: 'Core Concepts',
      items: [
        'core-concepts/what-are-feature-flags',
        'core-concepts/projects-and-environments',
        'core-concepts/flags',
        'core-concepts/targeting',
        'core-concepts/segments',
        'core-concepts/rollouts',
        'core-concepts/flag-lifecycle',
      ],
    },
    {
      type: 'category',
      label: 'Dashboard Guide',
      items: [
        'dashboard/managing-flags',
        'dashboard/lifecycle-dashboard',
        'dashboard/kill-switch-dashboard',
        'dashboard/audit-log',
        'dashboard/team-management',
        'dashboard/sso-oidc',
      ],
    },
    {
      type: 'category',
      label: 'SDKs',
      items: [
        'sdks/overview',
        'sdks/javascript',
        'sdks/react',
        'sdks/go',
        'sdks/dotnet',
      ],
    },
    {
      type: 'category',
      label: 'API Reference',
      items: [
        'api-reference/authentication',
        'api-reference/management-api',
        'api-reference/client-api',
      ],
    },
  ],
};

export default sidebars;
```

**Step 5: Create `docs-site/src/css/custom.css`**

```css
@import url('https://fonts.googleapis.com/css2?family=Sora:wght@300;400;500;600;700&family=Fira+Code:wght@400;500&display=swap');

:root {
  --ifm-color-primary: #d4956a;
  --ifm-color-primary-dark: #cc8355;
  --ifm-color-primary-darker: #c77a4a;
  --ifm-color-primary-darkest: #ab6236;
  --ifm-color-primary-light: #dcab83;
  --ifm-color-primary-lighter: #e0b48f;
  --ifm-color-primary-lightest: #ebd0b8;
  --ifm-font-family-base: 'Sora', system-ui, -apple-system, sans-serif;
  --ifm-font-family-monospace: 'Fira Code', SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  --ifm-code-font-size: 95%;
}

[data-theme='dark'] {
  --ifm-color-primary: #d4956a;
  --ifm-color-primary-dark: #cc8355;
  --ifm-color-primary-darker: #c77a4a;
  --ifm-color-primary-darkest: #ab6236;
  --ifm-color-primary-light: #dcab83;
  --ifm-color-primary-lighter: #e0b48f;
  --ifm-color-primary-lightest: #ebd0b8;
}
```

**Step 6: Create placeholder intro doc**

Create `docs-site/docs/getting-started/introduction.md`:

```markdown
---
slug: /
sidebar_position: 1
---

# Introduction

Placeholder — replaced in Task 2.
```

**Step 7: Add npm scripts to `docs-site/package.json`**

Ensure `package.json` has these scripts:
```json
{
  "scripts": {
    "start": "docusaurus start",
    "build": "docusaurus build",
    "clear": "docusaurus clear"
  }
}
```

**Step 8: Build to verify scaffolding works**

```bash
cd docs-site && npm run build
```

Expected: Successful build with output in `docs-site/build/`.

**Step 9: Commit**

```bash
git add docs-site/
git commit -m "docs: scaffold Docusaurus 3 site with theme and sidebar config"
```

---

### Task 2: Getting Started Section (3 pages)

**Files:**
- Create: `docs-site/docs/getting-started/introduction.md`
- Create: `docs-site/docs/getting-started/quick-start.md`
- Create: `docs-site/docs/getting-started/first-flag-in-code.md`

**Context to read before writing:**
- `README.md` — existing quick start content and SDK examples
- `docker-compose.yml` — exact Docker Compose config for quick start
- `docs/plans/2026-03-09-documentation-site-design.md` — content strategy

**Step 1: Write `introduction.md`**

Content covers:
- What Togglerino is (self-hosted feature flag platform, single Go binary)
- Key features list (flag types, targeting, rollouts, multi-environment, real-time SSE, team management, audit log, SDKs)
- Architecture overview (single binary serving dashboard + management API + SDK API + SSE)
- Brief mention of feature flags with link to `/core-concepts/what-are-feature-flags`
- "Next: Quick Start" link

Frontmatter: `slug: /`, `sidebar_position: 1`, `title: Introduction`

**Step 2: Write `quick-start.md`**

Content covers:
- Prerequisites (Docker and Docker Compose)
- Step 1: Start with Docker Compose (`docker compose up`)
- Step 2: Open `http://localhost:8090`, create admin account
- Step 3: Create a project (explain what projects are briefly, link to core concepts)
- Step 4: Create a boolean flag in the dashboard
- Step 5: Enable the flag in the development environment
- "Next: First Flag in Code" link

Frontmatter: `sidebar_position: 2`, `title: Quick Start`

**Step 3: Write `first-flag-in-code.md`**

Content covers:
- Prerequisites (completed Quick Start, have a flag and SDK key)
- Getting an SDK key: navigate to project → Environments → Development → SDK Keys → Create
- JavaScript SDK example: install, initialize, evaluate, listen for changes
- React SDK example: install, provider setup, useFlag hook
- Verify real-time updates: toggle flag in dashboard, see change in code via SSE
- Links to full SDK docs for Go and .NET

Frontmatter: `sidebar_position: 3`, `title: First Flag in Code`

**Step 4: Build to verify**

```bash
cd docs-site && npm run build
```

**Step 5: Commit**

```bash
git add docs-site/docs/getting-started/
git commit -m "docs: add Getting Started section (introduction, quick start, first flag)"
```

---

### Task 3: Self-Hosting Section (4 pages)

**Files:**
- Create: `docs-site/docs/self-hosting/installation.md`
- Create: `docs-site/docs/self-hosting/configuration.md`
- Create: `docs-site/docs/self-hosting/production.md`
- Create: `docs-site/docs/self-hosting/upgrading.md`

**Context to read before writing:**
- `Dockerfile` — exact build stages and base images
- `docker-compose.yml` — production-like Docker Compose config
- `internal/config/config.go` — all env vars, defaults, and validation
- `README.md` — existing "From Source" instructions

**Step 1: Write `installation.md`**

Content covers:
- **Docker (recommended)**: Pull from `ghcr.io/togglerino/togglerino:latest`, run with Docker Compose (include full `docker-compose.yml` example with PostgreSQL), mention port mapping (8090→8080 in compose, 8080 default)
- **Pre-built binary**: Download from GitHub Releases, run with `DATABASE_URL` env var, requires external PostgreSQL
- **Build from source**: Requirements (Go 1.25+, Node 20+, PostgreSQL), build frontend first (`cd web && npm install && npm run build`), then `go build -o togglerino ./cmd/togglerino`, run binary

Frontmatter: `sidebar_position: 1`, `title: Installation`

**Step 2: Write `configuration.md`**

Complete env var reference table with all variables from `internal/config/config.go`:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | `postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable` | PostgreSQL connection string |
| `CORS_ORIGINS` | `*` | Comma-separated allowed origins |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `SESSION_SECRET` | (auto-generated) | HMAC key for session/OIDC cookies. Set for persistence across restarts |
| `BASE_URL` | (auto-derived) | External base URL for OIDC callbacks |
| `OIDC_ISSUER_URL` | — | OIDC provider issuer URL |
| `OIDC_CLIENT_ID` | — | OIDC client ID |
| `OIDC_CLIENT_SECRET` | — | OIDC client secret |
| `OIDC_DEFAULT_ROLE` | `member` | Default role for OIDC-provisioned users (`admin` or `member`) |

Add notes on: CORS configuration (wildcard vs specific origins), `SESSION_SECRET` importance for multi-instance, `BASE_URL` for reverse proxy setups.

Frontmatter: `sidebar_position: 2`, `title: Configuration`

**Step 3: Write `production.md`**

Content covers:
- **Reverse proxy**: Togglerino designed to run behind nginx/Caddy for TLS. Include nginx example config (proxy_pass, WebSocket/SSE headers, timeouts for SSE). Include Caddy example (simpler).
- **TLS**: Terminate at reverse proxy, not in Togglerino. Mention setting `BASE_URL` when behind proxy.
- **PostgreSQL**: Recommend PostgreSQL 16+, connection pooling for high traffic, backup strategy. Note: migrations run automatically on startup.
- **Resource sizing**: Togglerino is lightweight (single binary, in-memory flag cache). Primary resource concern is SSE connections (one per connected SDK client).
- **Health checks**: `GET /healthz` returns `{"status":"ok"}`, use for load balancer health checks.

Frontmatter: `sidebar_position: 3`, `title: Production Deployment`

**Step 4: Write `upgrading.md`**

Content covers:
- Upgrade process: pull new image/binary, restart. Migrations run automatically on startup.
- Breaking changes: check CHANGELOG.md before upgrading.
- Rollback: if migration fails, restore from database backup. Migrations run in transactions so partial failures are safe.
- Docker: update image tag in compose file, `docker compose pull && docker compose up -d`.

Frontmatter: `sidebar_position: 4`, `title: Upgrading`

**Step 5: Build to verify**

```bash
cd docs-site && npm run build
```

**Step 6: Commit**

```bash
git add docs-site/docs/self-hosting/
git commit -m "docs: add Self-Hosting section (installation, configuration, production, upgrading)"
```

---

### Task 4: Core Concepts Section (7 pages)

**Files:**
- Create: `docs-site/docs/core-concepts/what-are-feature-flags.md`
- Create: `docs-site/docs/core-concepts/projects-and-environments.md`
- Create: `docs-site/docs/core-concepts/flags.md`
- Create: `docs-site/docs/core-concepts/targeting.md`
- Create: `docs-site/docs/core-concepts/segments.md`
- Create: `docs-site/docs/core-concepts/rollouts.md`
- Create: `docs-site/docs/core-concepts/flag-lifecycle.md`

**Context to read before writing:**
- `internal/model/flag.go` — flag types, value types, lifecycle statuses
- `internal/model/evaluation.go` — evaluation context, result, conditions, operators
- `internal/model/segment.go` — segment model
- `internal/evaluation/engine.go` — evaluation flow, consistent hashing, operator logic
- `internal/staleness/checker.go` — lifecycle transitions, default thresholds
- `docs/plans/2026-03-09-documentation-site-design.md` — content strategy

**Step 1: Write `what-are-feature-flags.md`**

Content covers:
- What feature flags are (configuration switches that control behavior without deploying new code)
- Why teams use them: decouple deployment from release, reduce risk, enable experimentation
- Common use cases with examples:
  - **Gradual rollouts** — release to 5% of users, then 25%, then 100%
  - **Kill switches** — instantly disable a broken feature in production
  - **A/B testing** — serve different variants to measure impact
  - **Ops toggles** — enable maintenance mode or debug logging
  - **Entitlements** — gate premium features by user plan
- How Togglerino's 5 flag types map to these use cases:
  - `release` — gradual feature rollouts (expected lifetime: ~40 days)
  - `experiment` — A/B tests and experiments (~40 days)
  - `operational` — ops toggles, maintenance mode (~7 days)
  - `kill-switch` — emergency circuit breakers (permanent)
  - `permission` — feature entitlements (permanent)
- Guidance on choosing the right type

Frontmatter: `sidebar_position: 1`, `title: What Are Feature Flags?`

**Step 2: Write `projects-and-environments.md`**

Content covers:
- **Projects**: top-level grouping for flags (typically one per application/service). Each project has a unique key.
- **Environments**: each project has multiple environments (e.g., development, staging, production). Default environments created automatically: `development`, `staging`, `production`.
- **Per-environment configuration**: each flag has separate config per environment (enabled/disabled, variants, targeting rules). A flag can be enabled in development but disabled in production.
- **SDK keys**: each environment has its own SDK keys. An SDK key scopes evaluation to a specific project + environment.

Frontmatter: `sidebar_position: 2`, `title: Projects & Environments`

**Step 3: Write `flags.md`**

Content covers:
- **Flag types** (5 types with descriptions — reference `what-are-feature-flags` for when to use each)
- **Value types**: `boolean` (true/false), `string` (any text), `number` (numeric), `json` (arbitrary JSON object/array)
- **Variants**: named value options for a flag. Every flag has at least a default variant. Multi-variant flags allow targeting rules to return different values.
- **Default value**: the fallback when no targeting rule matches or the flag is disabled.
- **Tags**: labels for organizing and filtering flags.
- **Creating a flag**: what fields are required (key, name, value type, flag type, default value).

Frontmatter: `sidebar_position: 3`, `title: Flags`

**Step 4: Write `targeting.md`**

Content covers:
- **Targeting rules**: ordered list of rules evaluated top-to-bottom, first match wins. Each rule has conditions (AND logic) and a variant to serve.
- **Conditions**: each condition has an attribute, operator, and value. Attributes come from the evaluation context (e.g., `user_id`, `country`, `plan`).
- **All 16 operators** with examples:

| Operator | Description | Example |
|----------|-------------|---------|
| `equals` | Exact match | `country equals "US"` |
| `not_equals` | Not equal | `plan not_equals "free"` |
| `contains` | Substring match | `email contains "@company.com"` |
| `not_contains` | No substring match | `email not_contains "@test.com"` |
| `starts_with` | Prefix match | `user_id starts_with "beta-"` |
| `ends_with` | Suffix match | `email ends_with ".edu"` |
| `greater_than` | Numeric greater | `age greater_than 18` |
| `less_than` | Numeric less | `age less_than 65` |
| `gte` | Greater or equal | `version gte 2.0` |
| `lte` | Less or equal | `version lte 3.5` |
| `in` | In set | `country in ["US", "CA", "UK"]` |
| `not_in` | Not in set | `country not_in ["CN", "RU"]` |
| `exists` | Attribute present | `user_id exists` |
| `not_exists` | Attribute absent | `internal_user not_exists` |
| `matches` | Regex match | `email matches ".*@company\\.com$"` |
| `segment_match` | Matches a segment | (see Segments page) |

- **Evaluation context**: the data SDKs send with each evaluation. Contains `user_id` and arbitrary `attributes`.

Frontmatter: `sidebar_position: 4`, `title: Targeting Rules & Conditions`

**Step 5: Write `segments.md`**

Content covers:
- What segments are: reusable groups of targeting conditions, scoped to a project. E.g., "Beta Users" segment with conditions `plan equals "beta"` AND `signup_date greater_than "2025-01-01"`.
- Using segments in targeting rules via the `segment_match` operator.
- Segments cannot reference other segments (no nesting).
- Deleting a segment is blocked if any flag references it (409 error).
- Managing segments in the dashboard (link to dashboard guide).

Frontmatter: `sidebar_position: 5`, `title: Segments`

**Step 6: Write `rollouts.md`**

Content covers:
- **Percentage rollouts**: a targeting rule can specify a percentage (0-100) instead of serving all matched users.
- **Consistent hashing**: Togglerino uses SHA-256 hash of `flagKey + userID`, mod 100. This ensures:
  - Same user always gets the same result for a given flag
  - Different flags roll out to different user subsets
  - Increasing the percentage from 10% to 20% keeps the original 10% included
- **How it works in practice**: Example — flag "new-checkout" with rule "country equals US, 25% rollout, variant: enabled". 25% of US users (determined by their user ID hash) see the new checkout.
- **No user ID**: if no `user_id` in context, rollout percentage is evaluated randomly per request (not sticky).

Frontmatter: `sidebar_position: 6`, `title: Percentage Rollouts`

**Step 7: Write `flag-lifecycle.md`**

Content covers:
- **State machine**: `active` → `potentially_stale` → `stale` → `archived`
- **Staleness checker**: runs hourly in the background, transitions flags based on age vs configured lifetime.
- **Default lifetimes** (per flag type):
  - `release`: 40 days
  - `experiment`: 40 days
  - `operational`: 7 days
  - `kill-switch`: permanent (never stale)
  - `permission`: permanent (never stale)
- **Per-project configuration**: admins can customize lifetime thresholds per flag type in project settings.
- **Manual overrides**: flag staleness status can be overridden manually (e.g., mark as "not stale" to reset the timer).
- **Archiving**: archived flags return their default value and are excluded from normal evaluation. Archive from the dashboard or API.

Frontmatter: `sidebar_position: 7`, `title: Flag Lifecycle & Staleness`

**Step 8: Build to verify**

```bash
cd docs-site && npm run build
```

**Step 9: Commit**

```bash
git add docs-site/docs/core-concepts/
git commit -m "docs: add Core Concepts section (feature flags, projects, flags, targeting, segments, rollouts, lifecycle)"
```

---

### Task 5: Dashboard Guide Section (6 pages)

**Files:**
- Create: `docs-site/docs/dashboard/managing-flags.md`
- Create: `docs-site/docs/dashboard/lifecycle-dashboard.md`
- Create: `docs-site/docs/dashboard/kill-switch-dashboard.md`
- Create: `docs-site/docs/dashboard/audit-log.md`
- Create: `docs-site/docs/dashboard/team-management.md`
- Create: `docs-site/docs/dashboard/sso-oidc.md`

**Context to read before writing:**
- `web/src/routes/` — route components to understand dashboard UI flows
- `internal/handler/management_handler.go` — API endpoints the dashboard calls
- `internal/model/` — permission types, roles, RBAC model
- `docs/plans/2026-03-09-documentation-site-design.md` — content strategy

**Step 1: Write `managing-flags.md`**

Content covers:
- **Creating a flag**: navigate to project → Flags → Create. Fill in key, name, description, value type, flag type, default value, tags.
- **Per-environment configuration**: click a flag → select environment tab. Toggle enabled/disabled. Set default variant.
- **Adding variants**: add named variants with different values. E.g., boolean flag with "on" (true) and "off" (false), or string flag with "control" and "treatment".
- **Adding targeting rules**: click "Add Rule", set conditions (attribute, operator, value), choose variant to serve, optionally set percentage rollout. Rules are ordered — drag to reorder. First match wins.
- **Archiving**: flag detail → Archive. Archived flags return their default value to all SDKs. Can be unarchived.
- Link to Core Concepts pages for targeting rules and flag types.

Frontmatter: `sidebar_position: 1`, `title: Managing Flags`

**Step 2: Write `lifecycle-dashboard.md`**

Content covers:
- **What it shows**: a board view of all flags organized by lifecycle status (active, potentially stale, stale, archived).
- **Purpose**: identify flags that should be cleaned up. Stale flags indicate tech debt.
- **Filtering**: filter by flag type, tag, or search.
- **Taking action**: click a flag to view details. Override staleness status if the flag is intentionally long-lived. Archive flags that are no longer needed.
- **Configuring lifetimes**: link to project settings for adjusting staleness thresholds per flag type.

Frontmatter: `sidebar_position: 2`, `title: Lifecycle Dashboard`

**Step 3: Write `kill-switch-dashboard.md`**

Content covers:
- **What it is**: a dedicated view for kill-switch type flags, designed for quick batch toggling during incidents.
- **Use case**: during an outage, quickly disable multiple features to reduce load or isolate a broken dependency.
- **How to use**: select environment, view all kill-switch flags with their current enabled/disabled state. Toggle multiple flags at once. Changes take effect immediately via SSE streaming.

Frontmatter: `sidebar_position: 3`, `title: Kill Switch Dashboard`

**Step 4: Write `audit-log.md`**

Content covers:
- **What's tracked**: flag create/update/delete, flag config changes (per-environment), project create/update/delete, lifecycle status changes. Each entry records who made the change, when, and full before/after JSON snapshots.
- **Viewing**: navigate to project → Audit Log. Entries are paginated (50 per page).
- **Best-effort**: audit log recording is best-effort — if it fails, the original action still succeeds.

Frontmatter: `sidebar_position: 4`, `title: Audit Log`

**Step 5: Write `team-management.md`**

Content covers:
- **Organization roles**: `admin` (full access, manage users/projects/OIDC) and `member` (access based on project roles).
- **Inviting users**: Admin → Team → Invite. Enter email and role. Generates a shareable invite link (expires in 7 days).
- **Project roles**: `admin` (full project access), `editor` (modify flags/environments/segments), `viewer` (read-only). Assigned per project.
- **Base project role**: organization-wide default project role for members (Settings → Base Project Role). Default: `editor`. Set to `none` to require explicit project membership.
- **Project members**: project → Settings → Members. Add/remove members, assign project-specific roles that override the base role.
- **Password reset**: admins can trigger password reset for any user (generates a reset link, expires 24 hours).
- **Deleting users**: admin only, removes user and all sessions.

Frontmatter: `sidebar_position: 5`, `title: Team Management & RBAC`

**Step 6: Write `sso-oidc.md`**

Content covers:
- **Overview**: Togglerino supports a single OIDC provider for SSO (e.g., Google, Okta, Auth0, Keycloak).
- **Configuration via dashboard**: Admin → Settings → SSO/OIDC. Enter issuer URL, client ID, client secret. Test the connection.
- **Configuration via env vars**: `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`. Env vars override database config.
- **How login works**: user clicks "Sign in with SSO" → redirected to provider → callback → session created.
- **Three callback outcomes**:
  1. Existing linked identity → session created directly
  2. Email matches existing user → prompted to enter password to link accounts
  3. New user → auto-provisioned with role from `OIDC_DEFAULT_ROLE` (default: `member`)
- **Account linking**: existing users can link their OIDC identity from Account → SSO Identities.
- **`SESSION_SECRET`**: must be set and stable for OIDC to work across restarts (HMAC-signed state cookies).

Frontmatter: `sidebar_position: 6`, `title: SSO / OIDC`

**Step 7: Build to verify**

```bash
cd docs-site && npm run build
```

**Step 8: Commit**

```bash
git add docs-site/docs/dashboard/
git commit -m "docs: add Dashboard Guide section (flags, lifecycle, kill switch, audit log, team, SSO)"
```

---

### Task 6: SDKs Section (5 pages)

**Files:**
- Create: `docs-site/docs/sdks/overview.md`
- Create: `docs-site/docs/sdks/javascript.md`
- Create: `docs-site/docs/sdks/react.md`
- Create: `docs-site/docs/sdks/go.md`
- Create: `docs-site/docs/sdks/dotnet.md`

**Context to read before writing:**
- `sdks/javascript/src/togglerino.ts` — client class, methods, config
- `sdks/javascript/src/types.ts` — all TypeScript types
- `sdks/react/src/` — provider, hooks
- `sdks/go/client.go`, `sdks/go/config.go`, `sdks/go/types.go` — Go SDK API
- `sdks/dotnet/src/Togglerino.Sdk/` — .NET client, options, models
- `sdks/dotnet/README.md` — existing .NET documentation
- `README.md` — existing JS/React examples

**Step 1: Write `overview.md`**

Content covers:
- All SDKs follow the same pattern: initialize with server URL + SDK key + optional context, evaluate flags, receive real-time updates via SSE.
- SDK comparison table:

| Feature | JavaScript | React | Go | .NET |
|---------|-----------|-------|-----|------|
| Package | `@togglerino/sdk` | `@togglerino/react` | `github.com/togglerino/togglerino/sdks/go` | `Togglerino.Sdk` |
| Streaming (SSE) | Yes | Yes (via JS SDK) | Yes | Yes |
| Polling fallback | Yes | Yes | Yes | Yes |
| Default poll interval | 30s | 30s | 30s | 30s |
| Bundle format | CJS + ESM | CJS + ESM | Go module | NuGet |

- **Common patterns**: evaluation context (`user_id` + arbitrary `attributes`), typed getters (`getBool`, `getString`, etc.), event subscriptions, context updates triggering re-evaluation.
- **Getting an SDK key**: project → Environments → select environment → SDK Keys → Create.

Frontmatter: `sidebar_position: 1`, `title: SDK Overview`

**Step 2: Write `javascript.md`**

Content covers:
- **Install**: `npm install @togglerino/sdk`
- **Initialize**:
  ```typescript
  import { Togglerino } from '@togglerino/sdk'

  const client = new Togglerino({
    serverUrl: 'https://flags.example.com',
    sdkKey: 'sdk_your_key_here',
    context: { userId: 'user-123', attributes: { plan: 'pro' } },
  })

  await client.initialize()
  ```
- **Evaluate flags**: `getBool(key, default)`, `getString(key, default)`, `getNumber(key, default)`, `getJson<T>(key, default)`, `getDetail(key)` (returns `{ value, variant, reason }`)
- **Events**: `on('ready', fn)`, `on('change', fn)`, `on('deleted', fn)`, `on('error', fn)`, `on('reconnecting', fn)`, `on('reconnected', fn)`, `on('context_change', fn)`. Returns unsubscribe function.
- **Update context**: `await client.updateContext({ userId: 'user-456' })` — re-fetches all flags with new context.
- **Configuration options**: `streaming` (default: true), `pollingInterval` (default: 30000ms).
- **Cleanup**: `client.close()` — stops SSE/polling and removes listeners.

Frontmatter: `sidebar_position: 2`, `title: JavaScript / TypeScript`

**Step 3: Write `react.md`**

Content covers:
- **Install**: `npm install @togglerino/react @togglerino/sdk`
- **Provider setup**:
  ```tsx
  import { TogglerioProvider } from '@togglerino/react'

  function App() {
    return (
      <TogglerioProvider config={{
        serverUrl: 'https://flags.example.com',
        sdkKey: 'sdk_your_key_here',
        context: { userId: 'user-123' },
      }}>
        <MyApp />
      </TogglerioProvider>
    )
  }
  ```
  Note: Provider renders `null` until the client is ready (flags fetched).
- **`useFlag` hook**: `const value = useFlag('flag-key', defaultValue)` — automatically re-renders on flag changes. Typed overloads for boolean, string, number, generic.
- **`useTogglerinoContext` hook**: `const { context, updateContext } = useTogglerinoContext()` — read and update evaluation context.
- **Full example**: a component that conditionally renders based on a flag and allows switching user context.

Frontmatter: `sidebar_position: 3`, `title: React`

**Step 4: Write `go.md`**

Content covers:
- **Install**: `go get github.com/togglerino/togglerino/sdks/go`
- **Initialize**:
  ```go
  import togglerino "github.com/togglerino/togglerino/sdks/go"

  client, err := togglerino.New(ctx, togglerino.Config{
      ServerURL: "https://flags.example.com",
      SDKKey:    "sdk_your_key_here",
      Context:   &togglerino.EvaluationContext{UserID: "user-123"},
  })
  defer client.Close()
  ```
- **Evaluate flags**: `BoolValue(key, default)`, `StringValue(key, default)`, `NumberValue(key, default)`, `JSONValue(key, target, default)`, `Detail(key)` (returns `EvaluationResult, bool`).
- **Events**: `OnChange(fn)`, `OnDeleted(fn)`, `OnError(fn)`, `OnReady(fn)`, `OnReconnecting(fn)`, `OnReconnected(fn)`, `OnContextChange(fn)`. Each returns unsubscribe function.
- **Update context**: `client.UpdateContext(ctx, &togglerino.EvaluationContext{UserID: "user-456"})`.
- **Configuration**: `Streaming` (default: true), `PollingInterval` (default: 30s), `HTTPClient` (custom), `Logger` (custom `slog.Logger`).

Frontmatter: `sidebar_position: 4`, `title: Go`

**Step 5: Write `dotnet.md`**

Content covers:
- **Install**: `dotnet add package Togglerino.Sdk`
- **Initialize**:
  ```csharp
  using Togglerino.Sdk;

  await using var client = new TogglerioClient(new TogglerioOptions
  {
      ServerUrl = "https://flags.example.com",
      SdkKey = "sdk_your_key_here",
      Context = new EvaluationContext { UserId = "user-123" },
  });

  await client.InitializeAsync();
  ```
- **Evaluate flags**: `GetBool(key, default)`, `GetString(key, default)`, `GetNumber(key, default)`, `GetJson<T>(key, default)`, `GetDetail(key)`.
- **Reactive events (IObservable<T>)**: `client.FlagChanges.Subscribe(e => ...)`, `client.FlagDeletions.Subscribe(...)`, `client.Errors.Subscribe(...)`.
- **Update context**: `await client.UpdateContextAsync(new EvaluationContext { UserId = "user-456" })`.
- **Configuration**: `Streaming` (default: true), `PollingInterval` (default: 30s). Optional `ILogger<TogglerioClient>` and `HttpClient` via constructor.
- **Disposal**: implements `IAsyncDisposable` and `IDisposable`. Use `await using` or `using`.

Frontmatter: `sidebar_position: 5`, `title: .NET`

**Step 6: Build to verify**

```bash
cd docs-site && npm run build
```

**Step 7: Commit**

```bash
git add docs-site/docs/sdks/
git commit -m "docs: add SDKs section (overview, JavaScript, React, Go, .NET)"
```

---

### Task 7: API Reference Section (3 pages)

**Files:**
- Create: `docs-site/docs/api-reference/authentication.md`
- Create: `docs-site/docs/api-reference/management-api.md`
- Create: `docs-site/docs/api-reference/client-api.md`

**Context to read before writing:**
- `cmd/togglerino/main.go` — all route registrations
- `internal/handler/management_handler.go` — management endpoint implementations
- `internal/handler/client_handler.go` — client/SDK endpoint implementations
- `internal/handler/auth_handler.go` — auth endpoint implementations
- `internal/model/` — request/response types

**Step 1: Write `authentication.md`**

Content covers:
- **Two authentication methods**: session-based (management dashboard) and SDK-key-based (client SDKs).
- **Session authentication**: POST `/api/v1/auth/login` with `{ email, password }`. Returns `Set-Cookie: session_id=...` (HttpOnly, SameSite=Lax, 7-day MaxAge). All management endpoints require this cookie. 401 if session expired/missing.
- **SDK key authentication**: pass SDK key in `Authorization: Bearer <sdk-key>` header. SDK keys are scoped to a project + environment. Used for `/api/v1/evaluate` and `/api/v1/stream` endpoints.
- **Rate limiting**: auth endpoints are rate-limited to 10 requests per 60 seconds per IP. Returns 429 with `Retry-After` header.
- **Initial setup**: when no users exist, POST `/api/v1/auth/setup` creates the first admin user. No authentication required.

Frontmatter: `sidebar_position: 1`, `title: Authentication`

**Step 2: Write `management-api.md`**

All session-authed endpoints, grouped by resource. Each endpoint documented with: method, path, description, required permissions (if any), example request body (if applicable), example response.

Groups:
- **Auth** (status, setup, login, logout, me, change-password, accept-invite, reset-password)
- **OIDC** (authorize, callback, link, config CRUD, identities)
- **Users** (list, invite, list invites, delete, reset password, project assignments)
- **Organization settings** (base project role)
- **Projects** (CRUD)
- **Project members** (list, add, update role, remove)
- **Environments** (create, list)
- **SDK Keys** (create, list, revoke)
- **Flags** (CRUD, per-env config, archive, staleness override, query params: `?tag=`, `?search=`)
- **Segments** (CRUD, usage)
- **Other** (unknown flags, context attributes, audit log, project settings)

Frontmatter: `sidebar_position: 2`, `title: Management API`

**Step 3: Write `client-api.md`**

Content covers:
- **Authentication**: all endpoints require `Authorization: Bearer <sdk-key>` header.
- **POST `/api/v1/evaluate`** — evaluate all flags:
  ```
  Request:  { "context": { "user_id": "user-123", "attributes": { "plan": "pro" } } }
  Response: { "flags": { "new-checkout": { "value": true, "variant": "enabled", "reason": "rule_match" } } }
  ```
- **POST `/api/v1/evaluate/{flagKey}`** — evaluate single flag:
  ```
  Request:  { "context": { "user_id": "user-123" } }
  Response: { "value": true, "variant": "enabled", "reason": "rule_match" }
  ```
- **GET `/api/v1/stream`** — SSE stream:
  - Auth via `Authorization: Bearer <sdk-key>` header (or query param for browser testing)
  - Initial event: `: connected` (keepalive comment)
  - Flag update events: `event: flag_update\ndata: {"flags": {...}}\n\n`
  - Clients should implement reconnection with backoff
- **Evaluation reasons**: `"default"` (no rule matched), `"rule_match"` (targeting rule matched), `"disabled"` (flag disabled in environment), `"archived"` (flag archived)

Frontmatter: `sidebar_position: 3`, `title: Client / Evaluation API`

**Step 4: Build to verify**

```bash
cd docs-site && npm run build
```

**Step 5: Commit**

```bash
git add docs-site/docs/api-reference/
git commit -m "docs: add API Reference section (authentication, management API, client API)"
```

---

### Task 8: CI/CD Integration

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`

**Step 1: Add `build-docs` job to `ci.yml`**

Add a new job after the existing jobs, before `build`:

```yaml
  build-docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: "20"

      - name: Build docs site
        run: cd docs-site && npm ci && npm run build
```

Update the `build` job `needs` array to include `build-docs`:

```yaml
  build:
    needs: [test-go, test-sdks, test-dotnet-sdk, test-go-sdk, lint-frontend, build-docs]
```

**Step 2: Add `deploy-docs` job to `release.yml`**

Add permissions needed for GitHub Pages at the workflow level (already has `contents: write`, `id-token: write` — add `pages: write`).

Add a new job after `docker`:

```yaml
  deploy-docs:
    runs-on: ubuntu-latest
    needs: release-please
    if: needs.release-please.outputs.release_created == 'true'
    permissions:
      pages: write
      id-token: write
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: "20"

      - name: Build docs site
        run: cd docs-site && npm ci && npm run build

      - uses: actions/upload-pages-artifact@v3
        with:
          path: docs-site/build

      - uses: actions/deploy-pages@v4
        id: deployment
```

**Step 3: Verify CI config is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); yaml.safe_load(open('.github/workflows/release.yml')); print('Valid YAML')"
```

If python3/yaml not available, use:
```bash
npx yaml-lint .github/workflows/ci.yml .github/workflows/release.yml
```

**Step 4: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m "ci: add docs site build validation and GitHub Pages deployment"
```

---

### Task 9: Update CLAUDE.md with Docs Convention

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add docs maintenance note to CLAUDE.md**

Add a new section after "## Other" at the end of CLAUDE.md:

```markdown
## Documentation Site

User-facing docs live in `docs-site/` (Docusaurus 3). Built and deployed to GitHub Pages on release.

```bash
cd docs-site && npm install && npm run build   # Build docs site
cd docs-site && npm start                      # Local dev server with hot reload
```

**Docs maintenance rule**: If you change API endpoints, env vars, UI flows, SDK interfaces, or flag evaluation behavior, update the relevant docs page in `docs-site/docs/`. Key mappings:
- Env vars (`internal/config/`) → `docs-site/docs/self-hosting/configuration.md`
- API routes (`internal/handler/`) → `docs-site/docs/api-reference/`
- SDK changes (`sdks/`) → `docs-site/docs/sdks/`
- Flag evaluation (`internal/evaluation/`) → `docs-site/docs/core-concepts/`
- Dashboard UI (`web/src/`) → `docs-site/docs/dashboard/`
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add docs site maintenance convention to CLAUDE.md"
```

---

### Task 10: Final Build Verification and Cleanup

**Files:**
- Possibly modify: any file with broken links or build errors

**Step 1: Clean build from scratch**

```bash
cd docs-site && rm -rf node_modules build .docusaurus && npm ci && npm run build
```

Expected: successful build with no warnings about broken links.

**Step 2: Verify all pages render**

```bash
cd docs-site && npm start
```

Manually check in browser:
- Landing page (introduction) loads
- All 6 sidebar categories expand and show correct pages
- All internal links work (click through a few)
- Light/dark mode toggle works
- Fonts (Sora, Fira Code) load correctly
- Amber accent color visible on links and active sidebar items

**Step 3: Check for broken internal links**

The `onBrokenLinks: 'throw'` and `onBrokenMarkdownLinks: 'throw'` config will cause build failures for broken links. If the build succeeds, all internal links are valid.

**Step 4: Commit any fixes**

```bash
git add -A docs-site/
git commit -m "docs: fix build issues from final verification"
```

(Only if there were fixes needed.)
