# Flag Evaluation Playground / Debugger

Issue: #34

## Summary

A playground that lets users test flag evaluation against a specific context without making real SDK calls. Shows the resolved value and a step-by-step rule trace explaining exactly why that result was returned.

## API

### `POST /api/v1/projects/{key}/playground`

Session-authed, requires `flags:read` project permission.

**Request:**
```json
{
  "environment_key": "production",
  "flag_key": "my-flag",
  "context": {
    "user_id": "user-123",
    "attributes": { "country": "DE", "plan": "pro" }
  }
}
```

- `flag_key` is optional. When omitted, evaluates all flags in the environment.
- `environment_key` is required.
- `context` is optional (defaults to empty context).

**Response:**
```json
{
  "results": [
    {
      "flag_key": "my-flag",
      "value": true,
      "variant": "enabled",
      "reason": "rule_match",
      "trace": {
        "steps": [
          { "type": "lifecycle_check", "status": "active", "passed": true },
          { "type": "enabled_check", "enabled": true, "passed": true },
          {
            "type": "rule",
            "rule_index": 0,
            "variant": "enabled",
            "percentage_rollout": 50,
            "hash_bucket": 23,
            "in_rollout": true,
            "matched": true,
            "conditions": [
              {
                "attribute": "country",
                "operator": "in",
                "condition_value": ["DE", "AT", "CH"],
                "actual_value": "DE",
                "passed": true
              },
              {
                "attribute": "plan",
                "operator": "segment_match",
                "condition_value": "premium-users",
                "passed": true,
                "segment_trace": [
                  {
                    "attribute": "plan",
                    "operator": "in",
                    "condition_value": ["pro", "enterprise"],
                    "actual_value": "pro",
                    "passed": true
                  }
                ]
              }
            ]
          },
          {
            "type": "rule",
            "rule_index": 1,
            "variant": "disabled",
            "matched": false,
            "skipped": true,
            "conditions": []
          }
        ],
        "default_variant": "disabled",
        "selected_step": 2
      }
    }
  ]
}
```

- Rules after the matched rule are marked `skipped: true` (shown for completeness, conditions not evaluated).
- `selected_step` is the index into `steps` that produced the final result.
- Segment conditions inlined under `segment_trace`.
- `hash_bucket` and `percentage_rollout` only present when rollout is configured.

## Backend

### Evaluation engine

New `EvaluateWithTrace` method on the `Engine` struct, parallel to existing `EvaluateWithSegments`. Returns `EvaluationTrace` containing the result plus ordered trace steps. The existing method is untouched — no performance impact on SDK evaluation.

### New types (`internal/model/`)

- `EvaluationTrace` — result + `[]TraceStep` + `DefaultVariant` + `SelectedStep`
- `TraceStep` — `Type` field (`lifecycle_check`, `enabled_check`, `rule`) plus type-specific fields
- `ConditionTrace` — attribute, operator, condition value, actual value, passed, optional `[]ConditionTrace` for segment expansion

### Handler

New `PlaygroundHandler` in `internal/handler/playground_handler.go`:
- Validates environment exists
- Fetches flags + segments from the in-memory cache
- Calls `engine.EvaluateWithTrace()` per flag
- Returns trace response

### Routing

```go
mux.Handle("POST /api/v1/projects/{key}/playground",
    wrap(playgroundHandler.Evaluate, sessionAuth, requireFlagsRead))
```

No new DB tables or migrations. Runs entirely against the in-memory cache.

## Frontend

### Routes

- `/projects/:key/playground` — main playground page
- Flag detail page gets a "Test this flag" link to `/projects/:key/playground?flag={flagKey}`

### Playground page

- **Form:** environment selector (dropdown), optional flag key (combobox with search), user ID text input, dynamic key-value attribute editor (add/remove rows)
- **"Evaluate" button** runs the POST, updates URL query params for shareability
- **Results:** card per flag showing value, variant, reason badge. Click to expand rule trace.

### Shareable URLs

Query params encode inputs: `?env=production&flag=my-flag&uid=user-123&attr.country=DE&attr.plan=pro`. Loading the page with params pre-fills the form and auto-evaluates.

### Trace visualization

- Vertical stepper/timeline showing each evaluation step
- Pass/fail/skip indicator per step (green check / red x / grey skip)
- Winning step highlighted with accent color
- Conditions as mini-table: attribute | operator | expected | actual | result
- Segment conditions nested under `segment_match` condition
- Hash bucket as small progress bar (0-100) when percentage rollout present

### Components

- `PlaygroundPage.tsx` — form + results layout
- `PlaygroundTrace.tsx` — trace timeline
- `PlaygroundCondition.tsx` — condition row

Uses existing shadcn components: card, input, select, badge, collapsible, button, table.

### API client

New function in `web/src/api/client.ts`:
```ts
playground.evaluate(projectKey, body) => POST /api/v1/projects/{key}/playground
```
