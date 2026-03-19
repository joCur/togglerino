---
sidebar_position: 4
title: Targeting Rules & Conditions
---

# Targeting Rules & Conditions

Targeting rules let you control which users see which variant of a flag. Instead of a simple on/off toggle, you can serve different values based on user attributes, geographic location, subscription plan, or any other context you provide.

## How Targeting Works

Each flag has a per-environment configuration that includes an ordered list of targeting rules. When an SDK evaluates a flag, the rules are checked top to bottom — the **first matching rule wins**.

If no rule matches, the flag returns its fallthrough variant.

## Evaluation Order

The full evaluation flow is:

1. **Archived check** — if the flag is archived, return the flag's default value immediately (reason: `archived`).
2. **Off variant** — if the flag is disabled in this environment, return the environment's `off_variant` (reason: `disabled`).
3. **Targeting rules** — evaluate each rule in order. The first rule whose conditions all match is selected.
4. **Percentage rollout** — if the matched rule has a percentage rollout, check whether the user falls within the rollout bucket. If not, continue to the next rule.
5. **Fallthrough variant** — if no rule matched, return the environment's `fallthrough_variant` (reason: `default`).

## Targeting Rules

Each targeting rule has three components:

- **Conditions** — one or more conditions that must **all** match (AND logic). If a rule has three conditions, all three must be true for the rule to apply.
- **Variant** — the variant to serve when the rule matches.
- **Percentage rollout** (optional) — a value from 0 to 100 that limits what fraction of matching users receive this variant. See [Percentage Rollouts](./rollouts) for details.

### Example

A flag `new-checkout` with three targeting rules evaluated in order:

| Priority | Conditions | Variant | Rollout |
|----------|-----------|---------|---------|
| 1 | `email` ends_with `"@yourcompany.com"` | `enabled` | 100% |
| 2 | `country` equals `"US"` | `enabled` | 25% |
| 3 | *(none)* | `disabled` | 100% |

- Internal employees (matching rule 1) always see the new checkout.
- 25% of US users (rule 2) see the new checkout.
- Everyone else gets the disabled variant (rule 3, the catch-all).

## Evaluation Context

The evaluation context is the data structure your SDK sends with each flag evaluation request. It tells Togglerino who the user is and what attributes they have.

```json
{
  "user_id": "user-abc-123",
  "attributes": {
    "email": "jane@example.com",
    "country": "US",
    "plan": "pro",
    "age": 28,
    "app_version": "2.4.1"
  }
}
```

- **`user_id`** (string) — identifies the user. Required for consistent [percentage rollouts](./rollouts). If omitted, rollouts become non-deterministic (random per request).
- **`attributes`** (key-value map) — arbitrary data used by targeting conditions. Keys are strings; values can be strings, numbers, booleans, or arrays.

Togglerino tracks which attribute names appear in evaluation contexts and surfaces them as autocomplete suggestions in the rule builder.

## Conditions

Each condition specifies three things:

- **Attribute** — the key to look up in the evaluation context's `attributes` map.
- **Operator** — how to compare the attribute value against the condition value.
- **Value** — the value to compare against.

### Operators

Togglerino supports 16 operators:

| Operator | Description | Example |
|----------|-------------|---------|
| `equals` | Exact string or number match | `country` equals `"US"` |
| `not_equals` | Not equal | `plan` not_equals `"free"` |
| `contains` | Substring match | `email` contains `"@company.com"` |
| `not_contains` | No substring match | `email` not_contains `"@test.com"` |
| `starts_with` | Prefix match | `user_id` starts_with `"beta-"` |
| `ends_with` | Suffix match | `email` ends_with `".edu"` |
| `greater_than` | Numeric greater than | `age` greater_than `18` |
| `less_than` | Numeric less than | `items_in_cart` less_than `10` |
| `gte` | Greater than or equal | `app_version` gte `2.0` |
| `lte` | Less than or equal | `app_version` lte `3.5` |
| `in` | Value is in a set | `country` in `["US", "CA", "UK"]` |
| `not_in` | Value is not in a set | `country` not_in `["CN", "RU"]` |
| `exists` | Attribute is present in context | `user_id` exists |
| `not_exists` | Attribute is absent from context | `internal_flag` not_exists |
| `matches` | Regular expression match | `email` matches `".*@company\\.com$"` |
| `segment_match` | Matches a reusable segment | See [Segments](./segments) |

### Condition Logic

Within a single rule, all conditions use **AND** logic — every condition must match for the rule to apply.

To achieve **OR** logic, create multiple rules. For example, to target users in the US *or* Canada, you can either:

- Use a single condition: `country` in `["US", "CA"]`
- Or create two separate rules, one for each country.
