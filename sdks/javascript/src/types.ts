/**
 * Configuration for the Togglerino SDK client.
 */
export interface TogglerinoConfig {
  /** Base URL of the Togglerino server (e.g. "http://localhost:8080"). */
  serverUrl: string

  /** SDK key for authenticating with the server. */
  sdkKey: string

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
 * SSE event emitted when a flag is deleted.
 */
export interface FlagDeletedEvent {
  flagKey: string
}

/**
 * Events emitted by the Togglerino client.
 * - "ready": fired after initial flag fetch completes.
 * - "change": fired when a flag value changes (via SSE or polling).
 * - "deleted": fired when a flag is deleted (via SSE). Payload is FlagDeletedEvent.
 * - "context_change": fired after updateContext() completes. Payload is EvaluationContext.
 * - "error": fired on fetch/SSE errors.
 * - "reconnecting": fired when scheduling an SSE reconnection attempt. Payload: { attempt: number, delay: number }.
 * - "reconnected": fired when SSE successfully reconnects after a disconnection.
 */
export type EventType = 'change' | 'deleted' | 'context_change' | 'error' | 'ready' | 'reconnecting' | 'reconnected'
