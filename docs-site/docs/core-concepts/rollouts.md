---
sidebar_position: 6
title: Percentage Rollouts
---

# Percentage Rollouts

Percentage rollouts let you release a feature to a fraction of your users, then gradually increase the percentage as you gain confidence. A targeting rule can include a rollout percentage (0-100) that determines what fraction of matching users receive the variant.

## How It Works

When a targeting rule has a percentage rollout set, the evaluation engine:

1. Checks if the user matches all of the rule's conditions.
2. If conditions match, computes a hash bucket for the user.
3. If the bucket falls within the rollout percentage, the user gets the rule's variant.
4. If the bucket is outside the percentage, the engine continues to the next rule (or falls back to the default variant).

## Consistent Hashing

Togglerino uses consistent hashing to assign users to rollout buckets. The algorithm:

1. Concatenate the flag key and user ID: `flagKey + userID`
2. Compute the SHA-256 hash of the concatenated string.
3. Take the first 8 bytes as a big-endian unsigned 64-bit integer.
4. Compute `integer mod 100` to get a bucket from 0 to 99.
5. If the bucket is less than the rollout percentage, the user is **in** the rollout.

### Why This Matters

**Sticky assignments.** The same user always gets the same result for a given flag. The hash is deterministic — if a user is in the 10% rollout today, they'll still be in it tomorrow. No database or session state required.

**Independent per flag.** Different flags produce different hash values (because the flag key is part of the input). A user in the 10% rollout for `new-checkout` won't necessarily be in the 10% rollout for `new-search`. This prevents the same group of users from always being the "guinea pigs."

**Incremental expansion.** When you increase a rollout from 10% to 20%, the original 10% of users stay included. The new 10% is additive — users with buckets 0-9 were already in, and now users with buckets 10-19 are added. No one "switches sides."

## Example

Flag `new-checkout` with the following targeting rule:

- **Conditions:** `country` equals `"US"`
- **Variant:** `enabled`
- **Percentage:** `25`

Result: 25% of US users (determined by SHA-256 of `"new-checkout"` + their user ID) receive the `enabled` variant. The other 75% of US users fall through to the next rule or the default variant.

To ramp up, change the percentage from 25 to 50. The original 25% of users stay in, and an additional 25% are added.

## No User ID

If the evaluation context does not include a `user_id`, the rollout percentage is applied non-deterministically — effectively random per request. This means the same user might get different results on consecutive requests.

For sticky rollouts, **always provide a user ID** in your evaluation context. This can be any stable identifier: a database user ID, session ID, device ID, or anonymous tracking ID.

## Full Rollout with Conditions

A targeting rule with conditions but no percentage rollout (or a percentage of 100) serves the variant to **all** users matching the conditions. This is useful for targeting specific segments without any gradual rollout:

| Conditions | Variant | Rollout |
|-----------|---------|---------|
| `plan` equals `"enterprise"` | `enabled` | *(none / 100%)* |

All enterprise users get the `enabled` variant immediately.
