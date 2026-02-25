/**
 * Configuration for the Togglerino SDK client.
 */
export interface TogglerinoConfig {
  /** Base URL of the Togglerino server (e.g. "http://localhost:8080"). */
  serverUrl: string

  /** SDK key for authenticating with the server. */
  sdkKey: string

  /** Project key (slug) to evaluate flags for. */
  project: string

  /** Environment key (e.g. "production", "development"). */
  environment: string

  /** Optional evaluation context (user, attributes). */
  context?: EvaluationContext

  /**
   * Whether to use SSE streaming for real-time flag updates.
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
 * Context passed to the server for flag evaluation (targeting rules).
 */
export interface EvaluationContext {
  /** Unique user identifier. Maps to "user_id" on the server. */
  userId?: string

  /** Arbitrary attributes for targeting (e.g. { plan: "pro", country: "US" }). */
  attributes?: Record<string, unknown>
}

/**
 * Result of evaluating a single flag.
 */
export interface EvaluationResult {
  value: unknown
  variant: string
  reason: string
}

/**
 * SSE event emitted when a flag changes.
 */
export interface FlagChangeEvent {
  flagKey: string
  value: unknown
  variant: string
}

/**
 * Events emitted by the Togglerino client.
 * - "ready": fired after initial flag fetch completes.
 * - "change": fired when a flag value changes (via SSE or polling).
 * - "error": fired on fetch/SSE errors.
 */
export type EventType = 'change' | 'error' | 'ready'
