import type {
  TogglerinoConfig,
  EvaluationContext,
  EvaluationResult,
  FlagChangeEvent,
  FlagDeletedEvent,
  EventType,
} from './types'

/**
 * Internal resolved config with all optional fields filled in.
 */
type ResolvedConfig = Required<Omit<TogglerinoConfig, 'context'>> & {
  context: EvaluationContext
}

/**
 * Shape of the response from POST /api/v1/evaluate
 */
interface EvaluateAllResponse {
  flags: Record<string, EvaluationResult>
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Listener = (...args: any[]) => void

/**
 * Togglerino SDK client.
 *
 * Usage:
 * ```ts
 * const client = new Togglerino({
 *   serverUrl: 'http://localhost:8080',
 *   sdkKey: 'sdk_abc123',
 *   context: { userId: 'user-42' },
 * })
 *
 * await client.initialize()
 *
 * if (client.getBool('dark-mode')) {
 *   enableDarkMode()
 * }
 *
 * client.on('change', (event) => {
 *   console.log('flag changed:', event.flagKey, event.value)
 * })
 * ```
 */
export class Togglerino {
  private config: ResolvedConfig
  private flags: Map<string, EvaluationResult> = new Map()
  private listeners: Map<string, Set<Listener>> = new Map()
  private pollTimer: ReturnType<typeof setInterval> | null = null
  private sseAbortController: AbortController | null = null
  private initialized = false
  private sseRetryCount = 0
  private sseRetryTimeout: ReturnType<typeof setTimeout> | null = null
  private readonly maxRetryDelay = 30000

  constructor(config: TogglerinoConfig) {
    this.config = {
      serverUrl: config.serverUrl.replace(/\/+$/, ''), // strip trailing slash
      sdkKey: config.sdkKey,
      context: config.context ?? {},
      streaming: config.streaming ?? true,
      pollingInterval: config.pollingInterval ?? 30_000,
    }
  }

  /**
   * Initialize the client: fetch all flags and start listening for updates.
   * Must be called before reading any flag values.
   */
  async initialize(): Promise<void> {
    await this.fetchFlags()
    this.initialized = true

    if (this.config.streaming) {
      this.startSSE()
    } else {
      this.startPolling()
    }

    this.emit('ready', undefined)
  }

  // ---------------------------------------------------------------------------
  // Typed flag getters (synchronous, read from local cache)
  // ---------------------------------------------------------------------------

  /**
   * Get a boolean flag value.
   * Returns `defaultValue` if the flag is not found or is not a boolean.
   */
  getBool(key: string, defaultValue = false): boolean {
    const result = this.flags.get(key)
    if (result === undefined || typeof result.value !== 'boolean') {
      return defaultValue
    }
    return result.value
  }

  /**
   * Get a string flag value.
   * Returns `defaultValue` if the flag is not found or is not a string.
   */
  getString(key: string, defaultValue = ''): string {
    const result = this.flags.get(key)
    if (result === undefined || typeof result.value !== 'string') {
      return defaultValue
    }
    return result.value
  }

  /**
   * Get a numeric flag value.
   * Returns `defaultValue` if the flag is not found or is not a number.
   */
  getNumber(key: string, defaultValue = 0): number {
    const result = this.flags.get(key)
    if (result === undefined || typeof result.value !== 'number') {
      return defaultValue
    }
    return result.value
  }

  /**
   * Get a JSON flag value (object, array, etc.).
   * Returns `defaultValue` if the flag is not found.
   */
  getJson<T = unknown>(key: string, defaultValue?: T): T {
    const result = this.flags.get(key)
    if (result === undefined) {
      return defaultValue as T
    }
    return result.value as T
  }

  /**
   * Get the raw EvaluationResult for a flag.
   * Returns undefined if the flag is not found.
   */
  getDetail(key: string): EvaluationResult | undefined {
    return this.flags.get(key)
  }

  // ---------------------------------------------------------------------------
  // Event emitter
  // ---------------------------------------------------------------------------

  /**
   * Subscribe to SDK events.
   * Returns an unsubscribe function.
   *
   * Events:
   * - "ready": no payload, fired after initialize() completes.
   * - "change": payload is FlagChangeEvent.
   * - "context_change": payload is EvaluationContext, fired after updateContext() completes.
   * - "error": payload is Error.
   * - "reconnecting": payload is { attempt: number, delay: number }, fired when scheduling SSE reconnect.
   * - "reconnected": no payload, fired when SSE successfully reconnects after a disconnection.
   */
  on(event: EventType, listener: Listener): () => void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set())
    }
    this.listeners.get(event)!.add(listener)

    // Return unsubscribe function
    return () => {
      this.listeners.get(event)?.delete(listener)
    }
  }

  // ---------------------------------------------------------------------------
  // Context management
  // ---------------------------------------------------------------------------

  /**
   * Get the current evaluation context.
   */
  getContext(): EvaluationContext {
    return {
      ...this.config.context,
      attributes: this.config.context.attributes
        ? { ...this.config.context.attributes }
        : undefined,
    }
  }

  /**
   * Update the evaluation context and re-fetch all flags.
   * Useful when the user logs in / changes attributes.
   */
  async updateContext(context: Partial<EvaluationContext>): Promise<void> {
    this.config.context = {
      ...this.config.context,
      ...context,
    }
    await this.fetchFlags()
    this.emit('context_change', this.getContext())
  }

  // ---------------------------------------------------------------------------
  // Cleanup
  // ---------------------------------------------------------------------------

  /**
   * Stop all background activity (SSE stream, polling) and remove listeners.
   * Call this when the client is no longer needed.
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
    this.listeners.clear()
  }

  // ---------------------------------------------------------------------------
  // Internal: flag fetching
  // ---------------------------------------------------------------------------

  /**
   * Fetch all flags from the server evaluation endpoint.
   */
  private async fetchFlags(): Promise<void> {
    const url = `${this.config.serverUrl}/api/v1/evaluate`

    const body = {
      context: {
        user_id: this.config.context.userId ?? '',
        attributes: this.config.context.attributes ?? {},
      },
    }

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${this.config.sdkKey}`,
        },
        body: JSON.stringify(body),
      })

      if (!response.ok) {
        throw new Error(
          `Togglerino: flag evaluation failed with status ${response.status}`
        )
      }

      const data: EvaluateAllResponse = await response.json()

      // Detect changes for emitting events
      const oldFlags = new Map(this.flags)

      this.flags.clear()
      for (const [key, result] of Object.entries(data.flags)) {
        this.flags.set(key, result)

        // Emit change events if the flag value has changed
        if (this.initialized) {
          const old = oldFlags.get(key)
          if (!old || JSON.stringify(old.value) !== JSON.stringify(result.value)) {
            this.emit('change', {
              flagKey: key,
              value: result.value,
              variant: result.variant,
            } satisfies FlagChangeEvent)
          }
        }
      }
    } catch (error) {
      this.emit('error', error)
      throw error
    }
  }

  // ---------------------------------------------------------------------------
  // Internal: SSE streaming (fetch-based with ReadableStream)
  // ---------------------------------------------------------------------------

  /**
   * Calculate the next retry delay using exponential backoff.
   * Sequence: 1s, 2s, 4s, 8s, 16s, 30s (capped).
   */
  private getRetryDelay(): number {
    const delay = Math.min(1000 * Math.pow(2, this.sseRetryCount), this.maxRetryDelay)
    this.sseRetryCount++
    return delay
  }

  /**
   * Schedule an SSE reconnection attempt with exponential backoff.
   * Starts polling as a fallback while retrying.
   */
  private scheduleSSEReconnect(): void {
    // Start polling as a fallback while we retry SSE
    if (this.pollTimer === null) {
      this.startPolling()
    }

    const delay = this.getRetryDelay()
    this.emit('reconnecting', { attempt: this.sseRetryCount, delay })

    this.sseRetryTimeout = setTimeout(() => {
      this.sseRetryTimeout = null
      this.startSSE()
    }, delay)
  }

  /**
   * Start an SSE connection using fetch + ReadableStream.
   * This allows us to send an Authorization header (unlike native EventSource).
   * On failure, schedules a reconnection attempt with exponential backoff
   * and uses polling as a fallback in the meantime.
   */
  private async startSSE(): Promise<void> {
    const url = `${this.config.serverUrl}/api/v1/stream`

    this.sseAbortController = new AbortController()

    const wasReconnecting = this.sseRetryCount > 0

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

      // SSE connection succeeded
      if (wasReconnecting) {
        this.emit('reconnected', undefined)
      }
      this.sseRetryCount = 0

      // Stop polling fallback since SSE is connected
      if (this.pollTimer !== null) {
        clearInterval(this.pollTimer)
        this.pollTimer = null
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()

      // Process SSE stream in the background.
      // When the stream ends (normally or via error), schedule reconnection.
      this.processSSEStream(reader, decoder).then(
        () => {
          // Stream ended normally (server closed connection) — reconnect
          this.scheduleSSEReconnect()
        },
        () => {
          // Stream errored — reconnect
          this.scheduleSSEReconnect()
        }
      )
    } catch {
      // SSE connection failed, schedule reconnection
      this.scheduleSSEReconnect()
    }
  }

  /**
   * Read and parse SSE events from a ReadableStream.
   * SSE format:
   *   event: flag_update
   *   data: {"flagKey":"dark-mode","value":true,"variant":"on"}
   *
   */
  private async processSSEStream(
    reader: ReadableStreamDefaultReader<Uint8Array>,
    decoder: TextDecoder
  ): Promise<void> {
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })

      // SSE events are separated by double newlines
      const parts = buffer.split('\n\n')
      // The last part may be incomplete
      buffer = parts.pop() ?? ''

      for (const part of parts) {
        this.handleSSEEvent(part)
      }
    }
  }

  /**
   * Parse a single SSE event block and update flags accordingly.
   */
  private handleSSEEvent(raw: string): void {
    let eventType = ''
    let data = ''

    for (const line of raw.split('\n')) {
      if (line.startsWith('event:')) {
        eventType = line.slice('event:'.length).trim()
      } else if (line.startsWith('data:')) {
        data = line.slice('data:'.length).trim()
      }
      // Lines starting with ":" are comments (keepalives), ignore them
    }

    if (!data) return

    if (eventType === 'flag_deleted') {
      try {
        const event: FlagDeletedEvent = JSON.parse(data)
        this.flags.delete(event.flagKey)
        this.emit('deleted', event)
      } catch {
        // Ignore malformed SSE data
      }
      return
    }

    if (eventType !== 'flag_update') return

    try {
      const event: FlagChangeEvent = JSON.parse(data)

      // Update the flag in the local cache
      const existing = this.flags.get(event.flagKey)
      this.flags.set(event.flagKey, {
        value: event.value,
        variant: event.variant,
        reason: existing?.reason ?? 'stream_update',
      })

      this.emit('change', event)
    } catch {
      // Ignore malformed SSE data
    }
  }

  // ---------------------------------------------------------------------------
  // Internal: polling fallback
  // ---------------------------------------------------------------------------

  /**
   * Start periodic polling as a fallback when SSE is unavailable.
   */
  private startPolling(): void {
    if (this.pollTimer !== null) return // already polling

    this.pollTimer = setInterval(() => {
      this.fetchFlags().catch(() => {
        // Errors already emitted via the 'error' event
      })
    }, this.config.pollingInterval)
  }

  // ---------------------------------------------------------------------------
  // Internal: event emission
  // ---------------------------------------------------------------------------

  private emit(event: string, payload: unknown): void {
    const set = this.listeners.get(event)
    if (!set) return
    for (const listener of set) {
      try {
        listener(payload)
      } catch {
        // Don't let listener errors break the SDK
      }
    }
  }
}
