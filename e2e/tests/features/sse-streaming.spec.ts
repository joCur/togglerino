import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

/** SSE event — now only carries type and flagKey (notification-only). */
interface SSEEvent {
  type: string;
  flagKey: string;
}

/**
 * Connect to the SSE stream and collect events until a condition is met or timeout.
 */
async function collectSSEEvents(
  sdkKey: string,
  opts: { timeout?: number; waitForFlagKey?: string } = {},
): Promise<SSEEvent[]> {
  const timeout = opts.timeout ?? 10_000;
  const events: SSEEvent[] = [];
  const controller = new AbortController();

  const res = await fetch(`${BASE_URL}/api/v1/stream`, {
    headers: { Authorization: `Bearer ${sdkKey}` },
    signal: controller.signal,
  });

  if (!res.ok || !res.body) {
    throw new Error(`SSE connection failed: ${res.status}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  const timeoutId = setTimeout(() => controller.abort(), timeout);

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const event = JSON.parse(line.slice(6));
            events.push(event);
            if (opts.waitForFlagKey && event.flagKey === opts.waitForFlagKey) {
              clearTimeout(timeoutId);
              controller.abort();
              return events;
            }
          } catch {
            // Skip non-JSON data lines
          }
        }
      }
    }
  } catch (e: any) {
    if (e.name !== 'AbortError') throw e;
  }

  clearTimeout(timeoutId);
  return events;
}

test.describe('SSE Streaming', () => {
  test('receives flag_update notification when flag config changes', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'SSE Test',
      value_type: 'boolean',
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'sse-test');

    // Start listening for events
    const eventsPromise = collectSSEEvents(sdkKey.key, { waitForFlagKey: flagKey, timeout: 10_000 });
    await new Promise(r => setTimeout(r, 500));

    // Toggle the flag — triggers SSE notification
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });

    const events = await eventsPromise;

    // Should receive a flag_update event with flagKey (notification-only, no value/variant)
    const flagEvents = events.filter(e => e.flagKey === flagKey);
    expect(flagEvents.length).toBeGreaterThanOrEqual(1);
    expect(flagEvents[0].type).toBe('flag_update');
    expect(flagEvents[0].flagKey).toBe(flagKey);
    // Events no longer carry value or variant — SDKs re-fetch from /evaluate
    expect((flagEvents[0] as any).value).toBeUndefined();
    expect((flagEvents[0] as any).variant).toBeUndefined();
  });

  test('SSE notification triggers correct re-evaluation', async ({ apiContext, testProject }) => {
    // This test verifies the full flow: SSE notifies → SDK re-fetches → gets correct value
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'SSE Re-eval',
      value_type: 'boolean',
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: false });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'sse-reeval');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Initially disabled → false
    const before = await client.evaluateFlag(flagKey);
    expect(before.value).toBe(false);

    // Start SSE listener
    const eventsPromise = collectSSEEvents(sdkKey.key, { waitForFlagKey: flagKey, timeout: 10_000 });
    await new Promise(r => setTimeout(r, 500));

    // Enable the flag
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });

    // Wait for SSE notification
    const events = await eventsPromise;
    expect(events.some(e => e.flagKey === flagKey)).toBeTruthy();

    // Re-fetch (as an SDK would) — should now return true
    const after = await client.evaluateFlag(flagKey);
    expect(after.value).toBe(true);
  });
});
