package togglerino

import (
	"encoding/json"
	"os"
	"testing"
)

// testCase mirrors the shared evaluation test fixture format.
type testCase struct {
	Name     string            `json:"name"`
	Flag     FlagDefinition    `json:"flag"`
	Segments []SegmentDefinition `json:"segments"`
	Context  struct {
		UserID     string         `json:"userId"`
		Attributes map[string]any `json:"attributes"`
	} `json:"context"`
	Expected struct {
		Value   any    `json:"value"`
		Variant string `json:"variant"`
		Reason  string `json:"reason"`
	} `json:"expected"`
}

func TestEvaluationEngine(t *testing.T) {
	data, err := os.ReadFile("../../testdata/evaluation_cases.json")
	if err != nil {
		t.Fatalf("failed to read test fixtures: %v", err)
	}

	var cases []testCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to parse test fixtures: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := EvaluationContext{
				UserID:     tc.Context.UserID,
				Attributes: tc.Context.Attributes,
			}

			result := evaluateFlag(tc.Flag, ctx, tc.Segments)

			// Compare values via JSON marshal for type-safe comparison
			// (e.g., float64 vs int, map vs struct).
			gotJSON, err := json.Marshal(result.Value)
			if err != nil {
				t.Fatalf("failed to marshal result value: %v", err)
			}
			wantJSON, err := json.Marshal(tc.Expected.Value)
			if err != nil {
				t.Fatalf("failed to marshal expected value: %v", err)
			}
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("value: got %s, want %s", gotJSON, wantJSON)
			}

			if result.Variant != tc.Expected.Variant {
				t.Errorf("variant: got %q, want %q", result.Variant, tc.Expected.Variant)
			}

			if result.Reason != tc.Expected.Reason {
				t.Errorf("reason: got %q, want %q", result.Reason, tc.Expected.Reason)
			}
		})
	}
}
