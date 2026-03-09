---
sidebar_position: 2
title: Projects & Environments
---

# Projects & Environments

Togglerino organizes feature flags into **projects** and **environments**, giving you a clear hierarchy from your application down to individual SDK clients.

## Projects

A project is the top-level organizational unit. Typically, you create one project per application or service — for example, "Web App", "Mobile API", or "Billing Service".

Each project has:

- **Name** — a human-readable label (e.g., "Web App")
- **Key** — a unique, URL-safe slug (e.g., `web-app`). Used in API paths and SDK configuration.
- **Its own flags, segments, environments, and settings** — everything is scoped to the project.

Projects are fully isolated. A flag key like `new-checkout` in one project is completely independent from a flag with the same key in another project.

## Environments

Each project has multiple environments representing your deployment stages. When you create a project, Togglerino automatically creates three default environments:

- **`development`** — for local development and testing. Flags are enabled by default in this environment.
- **`staging`** — for pre-production validation. Flags are disabled by default.
- **`production`** — for live traffic. Flags are disabled by default.

You can create additional environments to match your deployment pipeline (e.g., `qa`, `canary`, `eu-production`).

### Per-Environment Flag Configuration

Each flag has independent configuration per environment. This means you can:

- Enable a flag in `development` while keeping it disabled in `production`.
- Configure different targeting rules per environment (e.g., target internal users in staging, do a 10% rollout in production).
- Assign different default variants per environment.

This separation lets you test flags safely in lower environments before enabling them in production.

## SDK Keys

Each environment has its own SDK keys. An SDK key authenticates an SDK client and scopes its flag evaluations to a specific project and environment combination.

Key properties:

- **Scoped** — an SDK key is tied to exactly one project + environment pair. An SDK using a production key for "Web App" only sees flags configured for that project's production environment.
- **Multiple keys per environment** — you can create multiple SDK keys for the same environment. This is useful for key rotation: create a new key, update your application, then revoke the old key.
- **Server-side and client-side** — SDK keys are used by both server-side and client-side SDKs.

## How It Fits Together

```
Project ("Web App")
├── Environment: development
│   ├── SDK Key: dev_abc123...
│   └── Flag configs (enabled/disabled, targeting rules, variants)
├── Environment: staging
│   ├── SDK Key: stg_def456...
│   └── Flag configs
└── Environment: production
    ├── SDK Key: prd_ghi789...  (primary)
    ├── SDK Key: prd_jkl012...  (rotation)
    └── Flag configs
```

An SDK client connects with a single SDK key, which determines both the project and environment. When the client evaluates a flag, Togglerino looks up the flag's configuration for that specific environment and returns the appropriate value.
