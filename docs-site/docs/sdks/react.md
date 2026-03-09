---
sidebar_position: 3
title: React
---

# React SDK

The `@togglerino/react` package provides a React context provider and hooks for evaluating feature flags with automatic re-rendering on flag changes.

## Installation

```bash
npm install @togglerino/react @togglerino/sdk
```

Both the React bindings and the underlying JavaScript SDK are required. The packages ship CommonJS and ESM builds with full TypeScript declarations.

## Provider Setup

Wrap your application with `TogglerioProvider` to initialize the SDK and make flag evaluation available to all child components.

```tsx
import { TogglerioProvider } from '@togglerino/react'

function App() {
  return (
    <TogglerioProvider config={{
      serverUrl: 'https://flags.example.com',
      sdkKey: 'sdk_your_key_here',
      context: { userId: 'user-123' },
    }}>
      <MyApp />
    </TogglerioProvider>
  )
}
```

The `config` prop accepts the same `TogglerinoConfig` object as the [JavaScript SDK](./javascript.md#configuration) (with `serverUrl`, `sdkKey`, `context`, `streaming`, and `pollingInterval`).

**Important:** The provider renders `null` until the client is initialized (initial flags fetched). Your app will not render until initialization completes. This ensures flag values are always available when your components mount.

## `useFlag` Hook

The `useFlag` hook evaluates a flag and automatically re-renders the component when the flag value changes via SSE or polling.

```tsx
import { useFlag } from '@togglerino/react'

function MyComponent() {
  const showBanner = useFlag('show-banner', false)
  const theme = useFlag('app-theme', 'light')
  const maxItems = useFlag('max-items', 10)

  return showBanner ? <Banner theme={theme} max={maxItems} /> : null
}
```

### Signature

```typescript
function useFlag(key: string, defaultValue: boolean): boolean
function useFlag(key: string, defaultValue: string): string
function useFlag(key: string, defaultValue: number): number
function useFlag<T = unknown>(key: string, defaultValue: T): T
```

The return type is inferred from the `defaultValue` type:

- Pass a `boolean` default to get a `boolean` back.
- Pass a `string` default to get a `string` back.
- Pass a `number` default to get a `number` back.
- Pass any other type to get that type back (uses `getJson` under the hood).

If the flag is missing or does not match the expected type, the `defaultValue` is returned.

### Real-time Updates

`useFlag` subscribes to the `change` event from the underlying JavaScript SDK client. When a flag value changes (via SSE streaming or polling), only the components using that specific flag key will re-render.

## `useTogglerinoContext` Hook

The `useTogglerinoContext` hook provides access to the current evaluation context and a function to update it.

```tsx
import { useTogglerinoContext } from '@togglerino/react'

function UserSwitcher() {
  const { context, updateContext } = useTogglerinoContext()

  const switchUser = async (userId: string) => {
    await updateContext({ userId })
    // Flags automatically re-evaluated with new context
  }

  return (
    <div>
      <p>Current user: {context.userId}</p>
      <button onClick={() => switchUser('user-456')}>Switch User</button>
    </div>
  )
}
```

### Return Value

| Property | Type | Description |
|----------|------|-------------|
| `context` | `EvaluationContext` | Current evaluation context (reactive — updates when context changes) |
| `updateContext` | `(ctx: Partial<EvaluationContext>) => Promise<void>` | Merge new values into the context and re-fetch flags |

Calling `updateContext` merges the provided fields into the existing context, triggers a re-fetch of all flags, and causes any `useFlag` hooks to re-render if their values changed.

## Full Example

A complete example combining the provider, `useFlag`, and `useTogglerinoContext`:

```tsx
import { TogglerioProvider } from '@togglerino/react'
import { useFlag } from '@togglerino/react'
import { useTogglerinoContext } from '@togglerino/react'

function FeatureBanner() {
  const showBanner = useFlag('promo-banner', false)
  const bannerText = useFlag('banner-text', 'Check out our new features!')

  if (!showBanner) return null

  return (
    <div className="banner">
      <p>{bannerText}</p>
    </div>
  )
}

function UserInfo() {
  const { context, updateContext } = useTogglerinoContext()
  const maxItems = useFlag('max-items', 10)

  return (
    <div>
      <p>User: {context.userId}</p>
      <p>Max items: {maxItems}</p>
      <button onClick={() => updateContext({ userId: 'user-789' })}>
        Switch User
      </button>
    </div>
  )
}

export default function App() {
  return (
    <TogglerioProvider config={{
      serverUrl: 'https://flags.example.com',
      sdkKey: 'sdk_your_key_here',
      context: {
        userId: 'user-123',
        attributes: { plan: 'pro' },
      },
    }}>
      <FeatureBanner />
      <UserInfo />
    </TogglerioProvider>
  )
}
```

This renders nothing until the SDK is initialized, then displays the banner (if enabled) and user info. Clicking "Switch User" re-evaluates all flags for the new user, and components automatically re-render with updated values.
