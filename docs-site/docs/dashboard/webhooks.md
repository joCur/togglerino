---
sidebar_position: 7
title: Webhooks
---

# Webhooks

Webhooks let you notify external services when changes happen in your Togglerino project. When a subscribed event occurs (e.g., a flag is created or updated), Togglerino sends an HTTP POST request to your configured URL with a signed JSON payload.

## Setting Up a Webhook

1. Navigate to your project.
2. Go to **Settings** and open the **Webhooks** tab.
3. Click **Create Webhook**.
4. Fill in the details:
   - **Name** — a descriptive label (e.g., "Slack notifications" or "CI Pipeline").
   - **URL** — the HTTPS endpoint that will receive webhook deliveries.
   - **Event types** — select which events should trigger deliveries.
5. Click **Create**.

The webhook secret is displayed **only once** at creation time. Copy it immediately and store it securely — you will need it to verify webhook signatures.

:::caution
The webhook secret cannot be retrieved after the creation dialog is closed. If you lose it, delete the webhook and create a new one.
:::

## Event Types

| Event | Triggered when... |
|-------|-------------------|
| `flag.created` | A new flag is created |
| `flag.updated` | A flag's metadata (name, description, tags, type) is updated |
| `flag.deleted` | A flag is permanently deleted |
| `flag.archived` | A flag is archived or unarchived |
| `flag_config.updated` | A flag's per-environment configuration changes (enabled state, variants, targeting rules) |
| `segment.created` | A new segment is created |
| `segment.updated` | A segment's conditions are updated |
| `segment.deleted` | A segment is deleted |
| `environment.created` | A new environment is created |

## Managing Webhooks

From the **Webhooks** tab in project settings, you can:

- **Enable/disable** a webhook without deleting it (toggle the enabled switch).
- **Edit** a webhook's name, URL, or subscribed event types.
- **Delete** a webhook permanently (also deletes its delivery history).

## Testing a Webhook

To verify your endpoint is working:

1. Open the webhook detail page by clicking on a webhook.
2. Click **Send Test**.
3. Togglerino sends a synchronous `webhook.test` event to your URL and displays the result (status code, duration, success/failure).

Test deliveries are recorded in the delivery log like regular deliveries.

## Delivery Log

Each webhook has a delivery log showing all recent delivery attempts:

- **Event type** — which event triggered the delivery.
- **Status code** — the HTTP response code from your endpoint.
- **Success** — whether the delivery was accepted (2xx response).
- **Duration** — how long the request took.
- **Timestamp** — when the delivery was attempted.

Delivery logs are retained for **30 days** and then automatically cleaned up.

## Retry Behavior

When a delivery fails (non-2xx response or network error), Togglerino retries up to **3 times** with exponential backoff. Each retry attempt is recorded separately in the delivery log.

## Verifying Signatures

Every webhook delivery includes an `X-Togglerino-Signature` header containing an HMAC-SHA256 hex digest of the request body, computed with your webhook secret. Always verify this signature before processing a webhook to ensure the request came from Togglerino.

**Example (Node.js):**

```javascript
const crypto = require('crypto');

function verifySignature(body, secret, signature) {
  const expected = crypto
    .createHmac('sha256', secret)
    .update(body)
    .digest('hex');
  return crypto.timingSafeEqual(
    Buffer.from(expected),
    Buffer.from(signature)
  );
}
```

## Security Notes

- **SSRF protection**: Webhook URLs are validated to reject private/internal IP addresses.
- **HTTPS recommended**: While HTTP URLs are accepted, HTTPS is strongly recommended to protect your webhook secret and payload data in transit.
- **Secret rotation**: To rotate a webhook secret, delete the webhook and create a new one with the same configuration.

## Permissions

Creating, editing, and deleting webhooks requires the `project:settings` permission. Only project admins (or organization admins) have this permission by default.
