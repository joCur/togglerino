/**
 * Server-side SDK for Togglerino feature flags.
 *
 * Unlike the client SDK (`Togglerino`), this class fetches flag *definitions*
 * and evaluates them locally. Each call to `evaluate(context)` runs the
 * evaluation engine against the cached definitions, so a single instance can
 * serve many different users/requests without re-fetching.
 *
 * Usage:
 * ```ts
 * import { TogglerioServer } from '@togglerino/sdk/server'
 *
 * const server = new TogglerioServer({ serverUrl, sdkKey })
 * await server.initialize()
 *
 * const flags = server.evaluate({ userId: req.userId, attributes: { plan: 'pro' } })
 * flags.getBool('dark-mode', false)
 *
 * server.close()
 * ```
 */

import type { EvaluationContext, EvaluationResult } from './types'
import type { DefinitionsResponse, FlagDefinition, SegmentDefinition } from './server-types'
import { evaluate } from './engine'

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

/**
 * Configuration for the server-side SDK.
 */
export interface TogglerioServerConfig {
  /** Base URL of the Togglerino server (e.g. "http://localhost:8080"). */
  serverUrl: string

  /** SDK key for authenticating with the server. */
  sdkKey: string

  /**
   * Whether to use SSE streaming for real-time definition updates.
   * Falls back to polling if SSE connection fails.
   * @default true
   */
  streaming?: boolean

  /**
   * Polling interval in milliseconds. Used when streaming is disabled
   * or as a fallback when SSE connection fails.
   * @default 30000
   */
  pollingInterval?: number
}

/**
 * Lightweight object returned by `evaluate()` that provides typed getters
 * for reading flag values from a locally-evaluated result set.
 */
export interface FlagEvaluator {
  /** Get a boolean flag value. */
  getBool(key: string, defaultValue?: boolean): boolean
  /** Get a string flag value. */
  getString(key: string, defaultValue?: string): string
  /** Get a numeric flag value. */
  getNumber(key: string, defaultValue?: number): number
  /** Get a JSON flag value (object, array, etc.). */
  getJson<T = unknown>(key: string, defaultValue?: T): T
  /** Get the raw EvaluationResult for a flag. */
  getDetail(key: string): EvaluationResult | undefined
}

// ---------------------------------------------------------------------------
// Internal resolved config
// ---------------------------------------------------------------------------

type ResolvedConfig = Required<TogglerioServerConfig>

// ---------------------------------------------------------------------------
// TogglerioServer
// ---------------------------------------------------------------------------

export class TogglerioServer {
  private config: ResolvedConfig
  private definitions: FlagDefinition[] = []
  private segments: SegmentDefinition[] = []
  private pollTimer: ReturnType<typeof setInterval> | null = null
  private sseAbortController: AbortController | null = null
  private sseRetryCount = 0
  private sseRetryTimeout: ReturnType<typeof setTimeout> | null = null
  private readonly maxRetryDelay = 30000

  constructor(config: TogglerioServerConfig) {
    this.config = {
      serverUrl: config.serverUrl.replace(/\/+$/, ''),
      sdkKey: config.sdkKey,
      streaming: config.streaming ?? true,
      pollingInterval: config.pollingInterval ?? 30_000,
    }
  }

  // -------------------------------------------------------------------------
  // Lifecycle
  // -------------------------------------------------------------------------

  /**
   * Initialize the server SDK: fetch all flag definitions and start
   * listening for updates via SSE or polling.
   */
  async initialize(): Promise<void> {
    await this.fetchDefinitions()

    if (this.config.streaming) {
      this.startSSE()
    } else {
      this.startPolling()
    }
  }

  /**
   * Stop all background activity (SSE stream, polling timers).
   */
  close(): void {
    if (this.pollTimer !== null) {
      clearInterval(this.pollTimer)
      this.pollTimer = null
    }

    if (this.sseAbortController) {
      this.sseAbortController.abort()
      this.sseAbortController = null
    }

    if (this.sseRetryTimeout) {
      clearTimeout(this.sseRetryTimeout)
      this.sseRetryTimeout = null
    }

    this.sseRetryCount = 0
  }

  // -------------------------------------------------------------------------
  // Evaluation
  // -------------------------------------------------------------------------

  /**
   * Evaluate all cached flag definitions against the provided context.
   * Returns a `FlagEvaluator` with typed getters.
   */
  evaluate(context?: EvaluationContext): FlagEvaluator {
    const results = new Map<string, EvaluationResult>()

    for (const flag of this.definitions) {
      results.set(flag.key, evaluate(flag, context, this.segments))
    }

    return createFlagEvaluator(results)
  }

  // -------------------------------------------------------------------------
  // Internal: definitions fetching
  // -------------------------------------------------------------------------

  private async fetchDefinitions(): Promise<void> {
    const url = `${this.config.serverUrl}/api/v1/definitions`

    const response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${this.config.sdkKey}`,
      },
    })

    if (!response.ok) {
      throw new Error(
        `TogglerinoServer: definitions fetch failed with status ${response.status}`
      )
    }

    const data: DefinitionsResponse = await response.json()
    this.definitions = data.flags
    this.segments = data.segments
  }

  // -------------------------------------------------------------------------
  // Internal: SSE streaming (same patterns as client.ts)
  // -------------------------------------------------------------------------

  private getRetryDelay(): number {
    const delay = Math.min(1000 * Math.pow(2, this.sseRetryCount), this.maxRetryDelay)
    this.sseRetryCount++
    return delay
  }

  private scheduleSSEReconnect(): void {
    if (this.pollTimer === null) {
      this.startPolling()
    }

    const delay = this.getRetryDelay()
    this.sseRetryTimeout = setTimeout(() => {
      this.sseRetryTimeout = null
      this.startSSE()
    }, delay)
  }

  private async startSSE(): Promise<void> {
    const url = `${this.config.serverUrl}/api/v1/stream`

    this.sseAbortController = new AbortController()

    try {
      const response = await fetch(url, {
        headers: {
          Authorization: `Bearer ${this.config.sdkKey}`,
          Accept: 'text/event-stream',
        },
        signal: this.sseAbortController.signal,
      })

      if (!response.ok || !response.body) {
        this.scheduleSSEReconnect()
        return
      }

      // SSE connected — reset retry state
      this.sseRetryCount = 0

      // Stop polling fallback
      if (this.pollTimer !== null) {
        clearInterval(this.pollTimer)
        this.pollTimer = null
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()

      this.processSSEStream(reader, decoder).then(
        () => this.scheduleSSEReconnect(),
        () => this.scheduleSSEReconnect()
      )
    } catch {
      this.scheduleSSEReconnect()
    }
  }

  private async processSSEStream(
    reader: ReadableStreamDefaultReader<Uint8Array>,
    decoder: TextDecoder
  ): Promise<void> {
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })

      const parts = buffer.split('\n\n')
      buffer = parts.pop() ?? ''

      for (const part of parts) {
        this.handleSSEEvent(part)
      }
    }
  }

  /**
   * On any flag_update or flag_deleted event, re-fetch ALL definitions.
   * Unlike the client SDK which fetches a single flag's evaluation,
   * the server SDK needs the full definition set for local evaluation.
   */
  private handleSSEEvent(raw: string): void {
    let eventType = ''

    for (const line of raw.split('\n')) {
      if (line.startsWith('event:')) {
        eventType = line.slice('event:'.length).trim()
      }
    }

    if (eventType === 'flag_update' || eventType === 'flag_deleted') {
      this.fetchDefinitions().catch(() => {})
    }
  }

  // -------------------------------------------------------------------------
  // Internal: polling fallback
  // -------------------------------------------------------------------------

  private startPolling(): void {
    if (this.pollTimer !== null) return

    this.pollTimer = setInterval(() => {
      this.fetchDefinitions().catch(() => {})
    }, this.config.pollingInterval)
  }
}

// ---------------------------------------------------------------------------
// FlagEvaluator factory
// ---------------------------------------------------------------------------

function createFlagEvaluator(results: Map<string, EvaluationResult>): FlagEvaluator {
  return {
    getBool(key: string, defaultValue = false): boolean {
      const result = results.get(key)
      if (result === undefined || typeof result.value !== 'boolean') {
        return defaultValue
      }
      return result.value
    },

    getString(key: string, defaultValue = ''): string {
      const result = results.get(key)
      if (result === undefined || typeof result.value !== 'string') {
        return defaultValue
      }
      return result.value
    },

    getNumber(key: string, defaultValue = 0): number {
      const result = results.get(key)
      if (result === undefined || typeof result.value !== 'number') {
        return defaultValue
      }
      return result.value
    },

    getJson<T = unknown>(key: string, defaultValue?: T): T {
      const result = results.get(key)
      if (result === undefined) {
        return defaultValue as T
      }
      return result.value as T
    },

    getDetail(key: string): EvaluationResult | undefined {
      return results.get(key)
    },
  }
}
