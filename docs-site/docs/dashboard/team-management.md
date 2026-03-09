---
sidebar_position: 5
title: Team Management & RBAC
---

# Team Management & RBAC

Togglerino uses a two-tier role system: **organization roles** control global access, and **project roles** control what users can do within each project.

## Organization Roles

Every user has one organization-level role:

| Role | Access |
|------|--------|
| **admin** | Full access to everything: manage users, create/delete projects, configure OIDC, and all flags across all projects. Admins bypass all project-level permission checks. |
| **member** | Access determined by project roles. Can view and interact with projects they have been granted access to. |

## Project Roles

Within each project, users have a project-level role that determines their permissions:

| Role | Permissions |
|------|-------------|
| **admin** | Full project access: manage flags, environments, segments, SDK keys, project settings, and project members. |
| **editor** | Create and modify flags, environments, and segments. Cannot manage SDK keys, project settings, or project members. |
| **viewer** | Read-only access to flags, environments, and segments. |

Custom project roles can also be created with a specific set of permissions. See [Custom Roles](#custom-roles) below.

### Permission Reference

Project roles are defined by the following permissions:

| Permission | Description |
|------------|-------------|
| `flags:read` | View flags and their configurations |
| `flags:write` | Create, update, and delete flags; modify per-environment configs |
| `environments:read` | View environments |
| `environments:write` | Create and modify environments |
| `sdk_keys:manage` | Create and delete SDK keys |
| `segments:write` | Create, update, and delete segments |
| `templates:manage` | Create and manage flag templates |
| `project:settings` | Modify project settings and manage project members |

## Inviting Users

Only organization admins can invite new users.

1. Go to **Settings** (top-level, not project settings) and click **Team**.
2. In the **Invite Team Member** section, enter the user's email address and select an organization role (`admin` or `member`).
3. Click **Send Invite**.
4. A shareable invite link is generated. Copy and send it to the new team member.

The invite link expires after **7 days**. When the invitee clicks the link, they create their own password to complete account setup.

Pending invites are displayed in the **Pending Invites** section at the bottom of the Team page, showing the invited email, assigned role, and expiration date.

## Base Project Role

The **base project role** is an organization-wide default that determines the project access level for all members who do not have an explicit per-project role assignment.

To configure it:

1. Go to **Settings** (top-level).
2. Find the **Base Project Role** setting.
3. Choose a role:
   - **editor** (default) — all members can edit flags in every project unless overridden.
   - **viewer** — all members have read-only access to every project unless overridden.
   - **none** — members have no project access by default; they must be explicitly added to each project.

Organization admins are not affected by this setting — they always have full access.

## Per-Project Members

To override the base project role for specific users within a project:

1. Navigate to the project.
2. Go to **Settings** and open the **Members** tab.
3. Click **Add Project** (from the Team page) or manage members directly from project settings.
4. Assign a specific project role (`admin`, `editor`, `viewer`, or a custom role) to the user.

To revert a user to the base project role, remove their per-project assignment.

## Custom Roles

Admins can create custom project roles with a tailored set of permissions:

1. Go to **Settings** (top-level) and click **Roles**.
2. Click **Create Role**.
3. Enter a role name and select the permissions to include.
4. Save the role.

Custom roles can then be assigned to users as project roles, just like the built-in `admin`, `editor`, and `viewer` roles.

:::note
Built-in roles (`admin`, `editor`, `viewer`) cannot be modified or deleted. Custom roles cannot be deleted while they are assigned to any project member or set as the base project role.
:::

## Environment-Scoped Permissions

By default, a project role's write permissions apply to **all environments** in the project. Environment-scoped permissions let you restrict which environments a role can write to — for example, allowing a QA role to modify flag configs in staging but not production.

### Configuring Environment Access

1. Navigate to the project.
2. Go to **Settings** and open the **Environment Access** tab.
3. For each role, select which environments it can write to.
4. Save.

### How It Works

- **Unrestricted by default**: if no restrictions are configured for a role, it can write to all environments.
- **Only write operations are restricted**: read access is not affected. A restricted role can still view flag configurations in all environments.
- **Organization admins bypass all restrictions**: they always have full access to every environment.
- **Applies to per-environment flag config updates**: changing a flag's enabled state, variants, or targeting rules in a specific environment. Flag creation, deletion, and metadata updates are not environment-scoped.

### Example

A team has three environments: development, staging, and production.

- Role "developer" is restricted to **development** only — developers can freely toggle flags in dev but cannot touch staging or production configs.
- Role "qa" is restricted to **development** and **staging** — QA can test flag configurations in both environments.
- Role "admin" has no restrictions — full access to all environments (this is the default for the built-in admin role).

## Password Management

### Users Changing Their Own Password

1. Click your avatar or name in the top bar.
2. Go to **Account**.
3. Use the **Change Password** section to set a new password.

### Admins Resetting a User's Password

1. Go to **Settings** and click **Team**.
2. Find the user in the team members list.
3. Click **Reset Password**.
4. A password reset link is generated (expires in **24 hours**). Share it with the user.

## Deleting Users

Only organization admins can delete users.

1. Go to **Settings** and click **Team**.
2. Find the user in the team members list.
3. Click **Remove**.
4. Confirm the deletion.

This permanently removes the user account and terminates all their active sessions. The action cannot be undone.
