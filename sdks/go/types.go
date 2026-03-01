package togglerino

// EvaluationContext holds user identity and attributes for flag evaluation.
type EvaluationContext struct {
	UserID     string         `json:"user_id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// EvaluationResult is the server's response for a single flag evaluation.
type EvaluationResult struct {
	Value   any    `json:"value"`
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

// FlagChangeEvent is emitted when a flag value changes.
type FlagChangeEvent struct {
	FlagKey  string `json:"flagKey"`
	Value    any    `json:"value"`
	Variant  string `json:"variant"`
	OldValue any    `json:"-"`
}

// FlagDeletedEvent is emitted when a flag is deleted.
type FlagDeletedEvent struct {
	FlagKey string `json:"flagKey"`
}

// evaluateRequest is the POST body sent to /api/v1/evaluate.
type evaluateRequest struct {
	Context *evaluateContext `json:"context"`
}

// evaluateContext is the wire format for EvaluationContext.
type evaluateContext struct {
	UserID     string         `json:"user_id"`
	Attributes map[string]any `json:"attributes"`
}

// evaluateResponse is the response from POST /api/v1/evaluate.
type evaluateResponse struct {
	Flags map[string]*EvaluationResult `json:"flags"`
}

// sseEvent is a parsed SSE event from the stream.
type sseEvent struct {
	Type    string `json:"type"`
	FlagKey string `json:"flagKey"`
	Value   any    `json:"value"`
	Variant string `json:"variant"`
}
