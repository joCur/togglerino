---
sidebar_position: 3
title: First Flag in Code
---

# First Flag in Code

In this guide you will connect a client SDK to your Togglerino instance and evaluate the flag you created in the [Quick Start](./quick-start.md).

## Prerequisites

- A running Togglerino instance with a project and at least one boolean flag (follow the [Quick Start](./quick-start.md) if you haven't already)
- Node.js 18+ installed (for the JavaScript and React examples below)

## Get an SDK Key

SDK keys authenticate your application with Togglerino and scope it to a specific project and environment.

1. Open the Togglerino dashboard at **http://localhost:8090**
2. Navigate to your project
3. Go to **Environments** and select **development**
4. Click **SDK Keys** and then **Create**
5. Give the key a name (for example, "Local dev") and copy the generated key

Keep this key handy — you will use it in the code examples below.

## JavaScript SDK

Install the SDK:

```bash
npm install @togglerino/sdk
```

Initialize the client and evaluate your flag:

```typescript
import { Togglerino } from '@togglerino/sdk'

const client = new Togglerino({
  serverUrl: 'http://localhost:8090',
  sdkKey: 'your-sdk-key',
  context: { userId: 'user-123' },
})

await client.initialize()

if (client.getBool('new-checkout-flow', false)) {
  console.log('New checkout enabled!')
}

// Listen for real-time changes
client.on('change', ({ flagKey, value }) => {
  console.log(`${flagKey} changed to ${value}`)
})
```

The `initialize()` call fetches all flag values from the server and opens an SSE connection for real-time updates. The second argument to `getBool` is the default value returned if the flag does not exist or evaluation fails.

## React SDK

Install both the React SDK and the core SDK:

```bash
npm install @togglerino/react @togglerino/sdk
```

Wrap your application with the `TogglerioProvider` and use the `useFlag` hook in any component:

```tsx
import { TogglerioProvider, useFlag } from '@togglerino/react'

function App() {
  return (
    <TogglerioProvider config={{
      serverUrl: 'http://localhost:8090',
      sdkKey: 'your-sdk-key',
      context: { userId: 'user-123' },
    }}>
      <Checkout />
    </TogglerioProvider>
  )
}

function Checkout() {
  const newCheckout = useFlag('new-checkout-flow', false)
  return newCheckout ? <NewCheckout /> : <LegacyCheckout />
}
```

The provider handles initialization and SSE streaming automatically. When a flag changes on the server, the `useFlag` hook triggers a re-render with the new value — no manual subscription needed.

## Verify Real-Time Updates

With your application running and connected to Togglerino:

1. Open the Togglerino dashboard at **http://localhost:8090**
2. Navigate to your flag's detail page
3. Toggle the flag off (or change its variant) in the **development** environment and save

Your application will receive the update instantly via SSE. If you are using the JavaScript SDK with the `change` listener, you will see the log message appear immediately. If you are using the React SDK, the component will re-render with the new flag value automatically.

## Other SDKs

Want to use Go or .NET? Togglerino also provides official SDKs for both. See the [SDK documentation](/sdks/overview) for installation instructions, usage examples, and API references for all available SDKs.
