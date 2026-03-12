---
sidebar_position: 2
title: Managing Environments
---

# Managing Environments

Environments in Togglerino represent your deployment stages (development, staging, production, etc.). Each environment has its own independent flag configurations, SDK keys, and settings.

## Default Environments

When you create a new project, three environments are automatically created:

- **Development** — for local development and testing
- **Staging** — for pre-production validation
- **Production** — for live traffic

## Creating Custom Environments

To add environments beyond the defaults:

1. Navigate to your project.
2. Click the **Environments** tab.
3. Click **Create Environment**.
4. Enter a unique key (URL-safe slug, e.g., `qa`, `canary`) and a display name.
5. Click **Create**.

New environments are empty — existing flags are not automatically configured for them. You'll need to configure each flag individually for the new environment or promote configurations from an existing environment.

## Per-Environment Configuration

Each flag has independent configuration for every environment. You can:

- Enable a flag in one environment while disabling it in another
- Configure different targeting rules per environment
- Assign different variants and values per environment
- Use flag promotion to copy configurations forward through your pipeline

See [Managing Flags](/dashboard/managing-flags) for details on per-environment flag configuration.

## Ordering Environments

You can reorder environments to define your promotion flow. The order determines which environments can promote to which — you can only promote from an earlier environment to a later one.

From the **Environments** tab, drag environments to reorder them.

## Deleting Environments

You can delete any environment, including the defaults, using the **Environments** tab. When you delete an environment:

- All flag configurations for that environment are removed
- All SDK keys for that environment are revoked
- Any scheduled flag changes for that environment are cancelled
- SDKs using keys from the deleted environment will fail to authenticate (401 response)

### Important Constraints

- **A project must always have at least one environment** — if you only have one environment, you cannot delete it. Create an additional environment first if needed.
- **Confirmation dialog** — if the environment has active SDK keys, you'll be shown a confirmation warning listing the affected keys.
- **Permissions** — only users with `project:settings` permission (project admins) can delete environments.

### Deleting vs. Disabling

Deleting an environment is permanent and removes all associated data. If you want to temporarily disable testing in an environment, consider disabling individual flags instead.

## SDK Keys

Each environment has its own set of SDK keys. An SDK key authenticates an SDK client and determines which project and environment it accesses.

- Multiple SDK keys can exist per environment (useful for key rotation)
- Revoking a key immediately prevents SDKs using that key from authenticating
- Keys are scoped to exactly one project + environment pair

See [API Reference: SDK Keys](/api-reference/management-api#sdk-keys) for details on managing keys via API.

## Further Reading

- [Core Concepts: Projects & Environments](/core-concepts/projects-and-environments) — detailed architecture and design
- [Managing Flags](/dashboard/managing-flags) — per-environment flag configuration and promotion
- [API Reference: Environments](/api-reference/management-api#environments) — API endpoints
