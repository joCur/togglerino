---
sidebar_position: 6
title: SSO / OIDC
---

# SSO / OIDC

Togglerino supports Single Sign-On via a single OIDC (OpenID Connect) provider. This is compatible with any standard OIDC provider, including Google Workspace, Okta, Auth0, Keycloak, and Azure AD (Entra ID).

## Setup via the Dashboard

Only organization admins can configure SSO.

1. Go to **Settings** (top-level) and open the **SSO/OIDC** tab.
2. Fill in the following fields:

| Field | Description |
|-------|-------------|
| **Name** | A display name for the provider (shown on the login button). |
| **Issuer URL** | The OIDC issuer URL from your provider (e.g., `https://accounts.google.com`, `https://your-org.okta.com`). |
| **Client ID** | The OAuth client ID from your OIDC provider. |
| **Client Secret** | The OAuth client secret from your OIDC provider. |
| **Scopes** | The OIDC scopes to request (default: `openid email profile`). |
| **Default Role** | The organization role assigned to auto-provisioned users: `admin` or `member` (default: `member`). |
| **Enabled** | Toggle to enable or disable SSO login. |

3. Copy the **Redirect URI** displayed on the page (format: `https://your-togglerino-url/api/v1/auth/oidc/callback`) and configure it in your OIDC provider's allowed redirect URIs.
4. Click **Save**.

Once configured, users will see a **"Sign in with SSO"** button on the login page.

To remove OIDC configuration entirely, click **Delete OIDC Configuration**.

## Setup via Environment Variables

As an alternative to dashboard configuration, set these environment variables:

- `OIDC_ISSUER_URL` — OIDC issuer URL
- `OIDC_CLIENT_ID` — OAuth client ID
- `OIDC_CLIENT_SECRET` — OAuth client secret
- `OIDC_DEFAULT_ROLE` — Default role for auto-provisioned users: `admin` or `member` (default: `member`)

Environment variables override any database-stored OIDC configuration. See [Configuration](/self-hosting/configuration) for details on all environment variables.

## How Login Works

When a user clicks "Sign in with SSO":

1. They are redirected to the OIDC provider for authentication.
2. After authenticating, the provider redirects back to Togglerino's callback URL.
3. One of three outcomes occurs:

### Existing Linked Identity

If the user's OIDC identity has been previously linked to a Togglerino account, a session is created and they are logged in immediately.

### Email Matches an Existing User

If the OIDC email matches an existing Togglerino user who has not yet linked their OIDC identity, the user is prompted to enter their Togglerino password to confirm their identity. Once confirmed, the OIDC identity is linked to their account, and future SSO logins proceed without the password step.

### New User

If no matching account exists, a new Togglerino account is automatically provisioned. The user is assigned the organization role specified by the **Default Role** setting (or the `OIDC_DEFAULT_ROLE` environment variable). They are logged in immediately.

## Account Linking

Existing users can proactively link their OIDC identity without waiting to encounter the email-matching flow:

1. Click your avatar or name in the top bar.
2. Go to **Account**.
3. In the **SSO Identities** section, click **Link Account**.
4. Authenticate with the OIDC provider.

After linking, the user can sign in via SSO or with their email and password.

## Email Verification Requirement

Togglerino requires the OIDC provider to return an `email_verified: true` claim in the ID token. If the claim is missing or set to `false`, the login is rejected with an `oidc_email_not_verified` error.

This prevents account linking to the wrong user when an identity provider returns an unverified email address.

**Providers known to include `email_verified`:** Google Workspace, Okta, Auth0, Azure AD (Entra ID).

**Providers that may omit the claim:** Some enterprise SAML-to-OIDC bridges and self-hosted identity providers may not include `email_verified` in the ID token. If users from your provider are blocked, check the server logs for `oidc email not verified` warnings — the `email_verified_present` field will indicate whether the claim was missing entirely or explicitly set to `false`.

If your provider does not emit the `email_verified` claim, ensure it is configured to include this claim in the ID token or userinfo response. Most providers support adding custom claims via scope or claim mapping configuration.

## Important: SESSION_SECRET

The OIDC authentication flow uses HMAC-signed cookies for the state parameter and nonce. These cookies are signed using the `SESSION_SECRET` environment variable.

If `SESSION_SECRET` is not set, Togglerino auto-generates one at startup. However, this means the secret changes on every restart, which will cause any in-progress OIDC authentication flows to fail (the callback cannot verify the state cookie signed by the previous instance).

**For OIDC to work reliably, set `SESSION_SECRET` to a stable value and keep it consistent across restarts and deployments.** See [Configuration](/self-hosting/configuration) for details.
