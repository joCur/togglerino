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

Variants are the named value options a flag can return. Every flag has at least one variant.

For a simple boolean flag, you might have two variants:

| Variant Key | Value |
|-------------|-------|
| `on` | `true` |
| `off` | `false` |

For a string flag running an A/B test, you might have:

| Variant Key | Value |
|-------------|-------|
| `control` | `"old-ui"` |
| `treatment-a` | `"new-ui-dark"` |
| `treatment-b` | `"new-ui-light"` |

Targeting rules select which variant to serve to each user. When no rule matches or the flag is disabled, the **default variant** is returned.

## Default Variant

Each flag's per-environment configuration specifies a default variant. This is the fallback value returned when:

- No targeting rule matches the evaluation context.
- The flag is disabled in that environment.
- The flag is archived.

Setting a sensible default is important — it's what most users will see until you configure targeting rules.

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
