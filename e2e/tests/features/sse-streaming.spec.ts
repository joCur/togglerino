import { test, expect } from '../../helpers/fixtures.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

/**
 * Connect to the SSE stream and collect events until a condition is met or timeout.
 */
async function collectSSEEvents(
  sdkKey: string,
  opts: { timeout?: number; waitForFlagKey?: string } = {},
): Promise<Array<{ type: string; flagKey: string; value: unknown; variant: string }>> {
  const timeout = opts.timeout ?? 10_000;
  const events: Array<{ type: string; flagKey: string; value: unknown; variant: string }> = [];
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

      // Parse SSE events from buffer
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? ''; // Keep incomplete line in buffer

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
  test('receives flag update event when flag is toggled', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'SSE Test',
      value_type: 'boolean',
      default_value: true,
    });

    // Create SDK key and start listening for events
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'sse-test');

    // Start SSE listener, then toggle the flag
    const eventsPromise = collectSSEEvents(sdkKey.key, { waitForFlagKey: flagKey, timeout: 10_000 });

    // Small delay to ensure the SSE connection is established
    await new Promise(r => setTimeout(r, 500));

    // Toggle the flag — this should trigger an SSE event
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: '',
      variants: [],
    });

    const events = await eventsPromise;

    // Should have received at least one flag_update event for our flag
    const flagEvents = events.filter(e => e.flagKey === flagKey);
    expect(flagEvents.length).toBeGreaterThanOrEqual(1);
    expect(flagEvents[0].type).toBe('flag_update');
  });
});
