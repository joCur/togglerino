---
slug: /
sidebar_position: 1
title: Introduction
---

# Introduction

Togglerino is a self-hosted feature flag management platform. It comes with a management dashboard, a management API, an SDK evaluation API, and SSE streaming for real-time flag updates. Pair it with PostgreSQL and you have a complete feature flag system running on your own infrastructure.

## Key Features

- **Multiple flag value types** — boolean, string, number, and JSON flags to cover any use case
- **Targeting rules** — attribute-based conditions with 16 operators (`equals`, `contains`, `starts_with`, `matches`, `segment_match`, and more) to precisely control who sees what
- **Percentage rollouts** — consistent hashing ensures users get stable flag assignments across evaluations
- **Multi-environment support** — manage flag configurations independently per environment (development, staging, production)
- **Real-time SSE updates** — flag changes are pushed to connected SDKs instantly, no polling required
- **Reusable segments** — define named groups of targeting conditions and share them across flags
- **Team management with RBAC** — org-level roles (admin, member) and project-level roles (admin, editor, viewer) with granular permissions
- **Audit log** — every flag and project change is recorded with full before/after snapshots
- **Flag lifecycle management** — flags progress through active, potentially stale, stale, and archived states with configurable staleness thresholds per flag type
- **Client SDKs** — official SDKs for JavaScript, React, Go, and .NET with automatic SSE reconnection

## Architecture Overview

Togglerino comes with a built-in dashboard. There are two authentication paths:

- **Session-based auth** for the management dashboard — users log in with email/password or SSO, and a secure cookie tracks their session
- **SDK-key auth** for client SDKs — each environment has its own SDK keys that authenticate evaluation and streaming requests

All data is stored in PostgreSQL. Flag evaluations use an in-memory cache for fast responses without hitting the database on every SDK request.

```
┌─────────────────────────────────────────────┐
│                Togglerino                   │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │Dashboard │  │ Mgmt API │  │ SDK API  │  │
│  │          │  │ (session)│  │ (sdk-key)│  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  │
│       │              │              │        │
│       │     ┌────────┴────────┐     │        │
│       │     │  Flag Eval      │     │        │
│       │     │  Engine + Cache │     │        │
│       │     └────────┬────────┘     │        │
│       │              │         ┌────┴─────┐  │
│       │              │         │ SSE Hub  │  │
│       │              │         └──────────┘  │
│  ┌────┴──────────────┴──────────────────┐   │
│  │             PostgreSQL               │   │
│  └──────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

New to feature flags? Learn [what they are and when to use them](/core-concepts/what-are-feature-flags).

## Next Steps

Ready to get started? Head over to the [Quick Start](./quick-start.md) guide to have Togglerino running in minutes.
