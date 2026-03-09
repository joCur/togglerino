---
sidebar_position: 1
title: What Are Feature Flags?
---

# What Are Feature Flags?

Feature flags (also called feature toggles or feature switches) are configuration switches that let you change application behavior without deploying new code. Instead of shipping a change and hoping for the best, you wrap it in a flag and control when, where, and for whom the change is active.

## Why Teams Use Feature Flags

- **Decouple deployment from release** — deploy code to production at any time, then enable it when you're ready.
- **Reduce risk** — roll back a broken feature in seconds by flipping a flag, instead of rushing a hotfix through your CI/CD pipeline.
- **Progressive rollouts** — release to a small percentage of users first, monitor metrics, then expand.
- **Run experiments** — show different experiences to different users and measure which performs better.
- **Instant kill switches** — keep an emergency off-switch for critical integrations that may fail.
- **Entitlements and gating** — control access to features based on user plans, roles, or other attributes.

## Common Use Cases

### Gradual Rollouts

Release a new checkout flow to 5% of users. Monitor error rates and conversion metrics. If everything looks good, increase to 25%, then 50%, then 100%. If something breaks at any stage, dial back to 0% instantly.

### Kill Switches

Your application depends on a third-party payment provider. During an outage, flip a kill switch flag to route payments through a backup provider or show a friendly maintenance message — no deploy required.

### A/B Testing

Show two different onboarding flows to new users. Measure which flow has better activation rates. Once you have statistically significant results, remove the flag and ship the winning variant.

### Ops Toggles

Enable verbose logging in production to debug a customer issue, then disable it when you're done. Toggle maintenance mode on and off without touching your deployment pipeline.

### Entitlements

Gate premium features behind a flag that checks the user's subscription plan. Users on the "pro" plan see advanced analytics; users on "free" see an upgrade prompt.

## Flag Types in Togglerino

Togglerino provides five flag types, each designed for a specific use case. Choosing the right type matters because it determines how the system tracks the flag's lifecycle and when it's considered stale.

| Flag Type | Purpose | Expected Lifetime | When to Remove |
|-----------|---------|-------------------|----------------|
| `release` | Gradual feature rollouts | ~40 days | After the feature is fully rolled out to all users |
| `experiment` | A/B tests and experiments | ~40 days | After the experiment concludes and a winner is chosen |
| `operational` | Ops toggles, maintenance mode, debug switches | ~7 days | After the operational issue is resolved |
| `kill-switch` | Emergency circuit breakers | Permanent | Never — always keep available to flip in an emergency |
| `permission` | Feature entitlements and plan-based gating | Permanent | Never — tied to ongoing business logic |

**Choosing the right type:** pick the type that matches your flag's intent. A flag that gates a premium feature is a `permission` flag, not a `release` flag — even if you're "releasing" the feature. The type drives staleness tracking: `release` flags older than 40 days get flagged for cleanup, while `permission` flags are expected to live forever.

See [Flag Lifecycle & Staleness](./flag-lifecycle) for details on how Togglerino tracks flag health over time.
