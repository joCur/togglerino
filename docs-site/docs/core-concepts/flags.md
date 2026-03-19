---
sidebar_position: 3
title: Flags
---

# Flags

A flag is the core building block in Togglerino. It represents a configurable switch that controls application behavior at runtime.

## Flag Key

Every flag has a unique key within its project. The key is a URL-safe slug (e.g., `new-checkout-flow`, `enable-dark-mode`) and is what your SDKs use to evaluate the flag:

```typescript
const showNewCheckout = client.getBool('new-checkout-flow', false);
```

Choose descriptive, kebab-case keys. They cannot be changed after creation.

## Flag Types

Togglerino supports five flag types that describe the flag's purpose. The type determines staleness thresholds and helps your team understand the flag's intent at a glance.

| Type | Purpose |
|------|---------|
| `release` | Gradual feature rollouts |
| `experiment` | A/B tests and experiments |
| `operational` | Ops toggles, maintenance mode, debug switches |
| `kill-switch` | Emergency circuit breakers |
| `permission` | Feature entitlements and plan-based gating |

See [What Are Feature Flags?](./what-are-feature-flags) for detailed guidance on choosing the right type.

## Value Types

Each flag has a value type that determines what kind of data it returns to your application.

### `boolean`

True or false. The most common value type. Use for simple on/off feature switches.

```json
true
```

### `string`

Any text value. Use for A/B test variant names, feature tier labels, or configuration strings.

```json
"variant-a"
```

### `number`

A numeric value (integer or decimal). Use for thresholds, limits, rate values, or percentages.

```json
42
```

### `json`

An arbitrary JSON object or array. Use for complex configuration payloads that don't fit into a single value.

```json
{ "maxRetries": 3, "timeout": 5000, "features": ["search", "export"] }
```

## Variants

Every flag — regardless of value type — uses **variants** to define the values it can return. A variant has a name and a value matching the flag's value type.

**Boolean flags** have two implicit variants that are auto-created and cannot be modified:

| Variant name | Value |
|--------------|-------|
| `true` | `true` |
| `false` | `false` |

**String, number, and JSON flags** have user-defined variants. For example, a string flag running an A/B test might have:

| Variant name | Value |
|--------------|-------|
| `control` | `"old-ui"` |
| `treatment-a` | `"new-ui-dark"` |
| `treatment-b` | `"new-ui-light"` |

## Evaluation Flow

All flag types follow the same unified evaluation flow:

1. **Archived check** — if the flag is archived, the flag's `default_value` is returned immediately (reason: `archived`).
2. **Off variant** — if the flag is disabled in this environment, the environment's `off_variant` is returned (reason: `disabled`). For boolean flags this is the `false` variant. For other types you configure which variant to serve when targeting is off.
3. **Targeting rules** — evaluate each rule in order. The first rule whose conditions all match is applied, serving the rule's configured variant.
4. **Fallthrough variant** — if no targeting rule matched, the environment's `fallthrough_variant` is returned (reason: `default`). This is what most users see until you configure targeting rules.

The `off_variant` and `fallthrough_variant` are configured per environment, giving you independent control over each environment's behavior.

### Boolean flags

For boolean flags, the `off_variant` is always `false` and the `fallthrough_variant` defaults to `true`. Targeting rules can serve either the `true` or `false` variant to specific user segments. This is all you need for kill switches, feature gates, and operational toggles.

### String, number, and JSON flags

These value types require at least one variant to be defined. The `fallthrough_variant` specifies which variant to serve when no targeting rule matches. The `off_variant` specifies which variant to serve when targeting is disabled — typically your "safe" fallback value.

Setting a sensible `fallthrough_variant` is important — it is what most users see until you configure targeting rules.

## Tags

Tags are string labels for organizing and filtering flags. A flag can have multiple tags. Use them to group related flags or track initiatives:

- `checkout` — all flags related to the checkout experience
- `mobile` — flags specific to mobile clients
- `q1-initiative` — flags tied to a quarterly initiative

You can filter flags by tag in the dashboard and through the API query parameter `?tag=`.

## Environment Locking

A flag's per-environment configuration can be locked to prevent changes. This is useful during production freezes, incident response, or any time you want to ensure a flag's behavior remains stable.

When locked:

- Config updates, toggle changes, and promotions into the locked environment are rejected with a **409 Conflict** error.
- Archiving is blocked if the flag is locked in any environment.
- Scheduled changes for the locked environment are skipped by the schedule checker.
- SDK evaluation is **not affected** — the flag continues to serve its current value normally.

Locking requires the `project:settings` permission. See [Managing Flags](/dashboard/managing-flags#locking-a-flag) for dashboard instructions and the [Management API](/api-reference/management-api#lock-environment-config) for the API reference.
