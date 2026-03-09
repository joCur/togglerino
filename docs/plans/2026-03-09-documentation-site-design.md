# Documentation Site Design

**Issue:** #90
**Date:** 2026-03-09
**Status:** Approved

## Summary

User-facing documentation site for Togglerino using Docusaurus 3, deployed to GitHub Pages as part of the release process. Full coverage across 6 sections targeting both self-hosting operators and developers integrating SDKs.

## Decisions

| Decision | Choice |
|----------|--------|
| Framework | Docusaurus 3 (TypeScript config) |
| Location | `docs-site/` in monorepo |
| Structure | Flat docs, single sidebar, 6 categories |
| Content depth | Full coverage, all 6 sections |
| API Reference format | Hand-written Markdown |
| Screenshots | None at launch (follow-up: automated Playwright) |
| Docs maintenance | CLAUDE.md convention at launch (follow-up: CI drift checks) |
| Theme | Docusaurus classic with amber `#d4956a` accent, Sora/Fira Code fonts, light/dark toggle |
| Deployment | GitHub Pages via `release.yml` |
| Base URL | Default `https://togglerino.github.io/togglerino/` (custom domain later) |
| CI | Build validation in `ci.yml` on PRs |

## Site Structure

```
docs-site/
├── docs/
│   ├── getting-started/
│   │   ├── introduction.md
│   │   ├── quick-start.md
│   │   └── first-flag-in-code.md
│   ├── self-hosting/
│   │   ├── installation.md
│   │   ├── configuration.md
│   │   ├── production.md
│   │   └── upgrading.md
│   ├── core-concepts/
│   │   ├── what-are-feature-flags.md
│   │   ├── projects-and-environments.md
│   │   ├── flags.md
│   │   ├── targeting.md
│   │   ├── segments.md
│   │   ├── rollouts.md
│   │   └── flag-lifecycle.md
│   ├── dashboard/
│   │   ├── managing-flags.md
│   │   ├── lifecycle-dashboard.md
│   │   ├── kill-switch-dashboard.md
│   │   ├── audit-log.md
│   │   ├── team-management.md
│   │   └── sso-oidc.md
│   ├── sdks/
│   │   ├── overview.md
│   │   ├── javascript.md
│   │   ├── react.md
│   │   ├── go.md
│   │   └── dotnet.md
│   └── api-reference/
│       ├── authentication.md
│       ├── management-api.md
│       └── client-api.md
├── static/
│   └── img/
├── src/
│   └── css/custom.css
├── docusaurus.config.ts
├── sidebars.ts
├── package.json
└── tsconfig.json
```

## Content Strategy

### Getting Started

Three-page funnel optimized for zero-to-working-flag in one sitting:

1. **Introduction** — What Togglerino is, key features, architecture overview. Brief mention of feature flags with link to Core Concepts deep dive.
2. **Quick Start** — Docker Compose up, create admin user, create first project, create first flag. All via the dashboard UI.
3. **First Flag in Code** — Wire up a JavaScript/React SDK, evaluate the flag, see it toggle in real time via SSE streaming.

### Self-Hosting

Operator-focused documentation:

1. **Installation** — Three methods: Docker image (ghcr.io), pre-built binary, build from source.
2. **Configuration** — Complete env var reference table: variable, description, default, example.
3. **Production** — Reverse proxy setup (nginx/Caddy examples), TLS termination, PostgreSQL recommendations, resource sizing.
4. **Upgrading** — Upgrade process, note that migrations run automatically on startup, breaking change notes.

### Core Concepts

Domain model explanations, each page standalone:

1. **What Are Feature Flags?** — What feature flags are, why teams use them, common use cases (gradual rollouts, kill switches, A/B testing, ops toggles, entitlements). How Togglerino's 5 flag types map to these use cases with guidance on when to pick each one.
2. **Projects & Environments** — Multi-tenancy model, default environments (development, staging, production), per-environment flag configuration.
3. **Flags** — 5 flag types (release, experiment, operational, kill-switch, permission), 4 value types (boolean, string, number, json), variants, default values.
4. **Targeting** — Rules (first match wins), conditions, all 16 operators with examples.
5. **Segments** — Reusable targeting condition groups, segment_match operator, no nesting.
6. **Rollouts** — Percentage rollouts, consistent hashing (SHA-256 of flagKey+userID mod 100), what users experience.
7. **Flag Lifecycle** — State machine (active → potentially_stale → stale → archived), staleness thresholds, per-project configuration, manual overrides.

### Dashboard Guide

Procedure-focused, no screenshots at launch:

1. **Managing Flags** — Create, configure per-environment, add targeting rules, set variants, archive.
2. **Lifecycle Dashboard** — Staleness board, filtering, bulk actions.
3. **Kill Switch Dashboard** — Batch kill switch configuration.
4. **Audit Log** — What's tracked, filtering, entity state snapshots.
5. **Team Management** — Invite users, assign org roles (admin/member), project roles (admin/editor/viewer), base project role setting.
6. **SSO/OIDC** — Configure via admin UI or env vars, authorization code flow, account linking, auto-provisioning.

### SDKs

Overview plus per-SDK pages following a consistent template:

1. **Overview** — SDK comparison table (language, streaming, bundle format), common patterns (initialization, evaluation, context).
2. **JavaScript** — Install, initialize, evaluate, streaming, configuration, API reference. Draws from main README examples.
3. **React** — Provider setup, useFlag hook, examples. Draws from main README examples.
4. **Go** — Install, initialize, evaluate, streaming/polling, configuration.
5. **.NET** — Install, initialize, evaluate, IObservable events, Polly resilience, configuration. Draws from existing .NET README.

### API Reference

Hand-written Markdown, grouped by auth type:

1. **Authentication** — Session cookies (management UI) vs SDK keys (client SDKs), how to obtain each.
2. **Management API** — All session-authed endpoints grouped by resource (auth, users, projects, environments, flags, segments, settings, audit log). Each endpoint: method, path, description, request/response examples.
3. **Client API** — Evaluate (single + batch) and SSE stream endpoints with example payloads and context format.

## Theming

Docusaurus classic theme with custom CSS overrides:

- **Primary color:** amber `#d4956a` (applied to links, buttons, sidebar active items)
- **Fonts:** Sora (sans-serif, headings + body), Fira Code (monospace, code blocks)
- **Mode:** Light/dark toggle (Docusaurus default), both modes supported
- Import fonts via Google Fonts in custom CSS

## CI/CD Integration

### Deployment (`release.yml`)

New `deploy-docs` job added after the existing release job:

1. Checkout repo
2. Install Node, run `npm ci && npm run build` in `docs-site/`
3. Upload artifact via `actions/upload-pages-artifact`
4. Deploy via `actions/deploy-pages`

Only runs when a release is created, keeping docs in sync with published versions.

### Validation (`ci.yml`)

New `build-docs` job that runs `npm ci && npm run build` in `docs-site/` on PRs. Catches broken links and build errors before merge. No deployment.

## Docs Maintenance

### At Launch

Update CLAUDE.md with convention: "If you changed API endpoints, env vars, UI flows, or SDK interfaces, update the relevant docs page in `docs-site/docs/`."

### Follow-up (separate issues)

- **Automated screenshots:** Playwright script that boots Togglerino in Docker, seeds data, captures screenshots. Run as part of release workflow.
- **CI drift checks:** Lint checks that verify env vars in `config.go` appear in `configuration.md`, API routes in handlers appear in API reference, etc.

## Docusaurus Configuration

Key settings in `docusaurus.config.ts`:

- `url`: `https://togglerino.github.io`
- `baseUrl`: `/togglerino/`
- `organizationName`: `togglerino`
- `projectName`: `togglerino`
- `docs.routeBasePath`: `/` (docs as landing page, no separate blog/landing)
- `themeConfig.navbar`: Togglerino logo + GitHub link
- `themeConfig.colorMode`: light default, dark available, user preference respected
