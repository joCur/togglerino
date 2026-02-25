package evaluation

import (
	"testing"
)

func TestEvaluateCondition_Equals(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"string match", "hello", "hello", true},
		{"string mismatch", "hello", "world", false},
		{"int match", 42, 42, true},
		{"int to string cross-type", 42, "42", true},
		{"float match", 3.14, 3.14, true},
		{"float to string cross-type", 3.14, "3.14", true},
		{"empty strings", "", "", true},
		{"nil vs nil", nil, nil, true},
		{"nil vs string", nil, "hello", false},
		{"bool true", true, true, true},
		{"bool false", false, false, true},
		{"bool mismatch", true, false, false},
		{"zero value int", 0, 0, true},
		{"zero int vs string", 0, "0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "equals", tt.cond)
			if got != tt.want {
				t.Errorf("equals(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_NotEquals(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"different strings", "hello", "world", true},
		{"same strings", "hello", "hello", false},
		{"different types same value", 42, "42", false},
		{"different values", 42, 43, true},
		{"empty vs non-empty", "", "hello", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "not_equals", tt.cond)
			if got != tt.want {
				t.Errorf("not_equals(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_Contains(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"string contains substring", "hello world", "world", true},
		{"string does not contain", "hello world", "foo", false},
		{"empty string contains empty", "", "", true},
		{"string contains empty", "hello", "", true},
		{"slice contains value", []any{"a", "b", "c"}, "b", true},
		{"slice does not contain", []any{"a", "b", "c"}, "d", false},
		{"slice contains int as string", []any{1, 2, 3}, "2", true},
		{"empty slice", []any{}, "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "contains", tt.cond)
			if got != tt.want {
				t.Errorf("contains(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_NotContains(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"string not contains", "hello world", "foo", true},
		{"string contains", "hello world", "world", false},
		{"slice not contains", []any{"a", "b"}, "c", true},
		{"slice contains", []any{"a", "b"}, "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "not_contains", tt.cond)
			if got != tt.want {
				t.Errorf("not_contains(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_StartsWith(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"starts with prefix", "hello world", "hello", true},
		{"does not start with", "hello world", "world", false},
		{"empty prefix", "hello", "", true},
		{"empty string empty prefix", "", "", true},
		{"full string match", "hello", "hello", true},
		{"number as string", 12345, "123", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "starts_with", tt.cond)
			if got != tt.want {
				t.Errorf("starts_with(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_EndsWith(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"ends with suffix", "hello world", "world", true},
		{"does not end with", "hello world", "hello", false},
		{"empty suffix", "hello", "", true},
		{"full string match", "hello", "hello", true},
		{"number as string", 12345, "345", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "ends_with", tt.cond)
			if got != tt.want {
				t.Errorf("ends_with(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_GreaterThan(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"int greater", 10, 5, true},
		{"int not greater", 5, 10, false},
		{"int equal", 5, 5, false},
		{"float greater", 3.14, 2.71, true},
		{"string numbers", "10", "5", true},
		{"mixed int and float", 10, 5.5, true},
		{"mixed string and int", "10", 5, true},
		{"non-numeric string", "abc", 5, false},
		{"zero greater than negative", 0, -1, true},
		{"negative less than zero", -1, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "greater_than", tt.cond)
			if got != tt.want {
				t.Errorf("greater_than(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_LessThan(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"int less", 5, 10, true},
		{"int not less", 10, 5, false},
		{"int equal", 5, 5, false},
		{"float less", 2.71, 3.14, true},
		{"string numbers", "5", "10", true},
		{"non-numeric string", "abc", 5, false},
		{"zero less than positive", 0, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "less_than", tt.cond)
			if got != tt.want {
				t.Errorf("less_than(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_GTE(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"int greater", 10, 5, true},
		{"int equal", 5, 5, true},
		{"int less", 5, 10, false},
		{"float equal", 3.14, 3.14, true},
		{"string number equal", "5", "5", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "gte", tt.cond)
			if got != tt.want {
				t.Errorf("gte(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_LTE(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"int less", 5, 10, true},
		{"int equal", 5, 5, true},
		{"int greater", 10, 5, false},
		{"float equal", 3.14, 3.14, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "lte", tt.cond)
			if got != tt.want {
				t.Errorf("lte(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_In(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"value in list", "b", []any{"a", "b", "c"}, true},
		{"value not in list", "d", []any{"a", "b", "c"}, false},
		{"int in list of strings", 42, []any{"41", "42", "43"}, true},
		{"int in list of ints", 42, []any{41, 42, 43}, true},
		{"empty list", "a", []any{}, false},
		{"nil condition", "a", nil, false},
		{"non-slice condition", "a", "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "in", tt.cond)
			if got != tt.want {
				t.Errorf("in(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_NotIn(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"value not in list", "d", []any{"a", "b", "c"}, true},
		{"value in list", "b", []any{"a", "b", "c"}, false},
		{"empty list", "a", []any{}, true},
		{"non-slice condition", "a", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "not_in", tt.cond)
			if got != tt.want {
				t.Errorf("not_in(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_Exists(t *testing.T) {
	tests := []struct {
		name string
		attr any
		want bool
	}{
		{"non-nil string", "hello", true},
		{"non-nil int", 42, true},
		{"non-nil zero", 0, true},
		{"non-nil empty string", "", true},
		{"non-nil false", false, true},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "exists", nil)
			if got != tt.want {
				t.Errorf("exists(%v) = %v, want %v", tt.attr, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_NotExists(t *testing.T) {
	tests := []struct {
		name string
		attr any
		want bool
	}{
		{"nil", nil, true},
		{"non-nil", "hello", false},
		{"zero value", 0, false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "not_exists", nil)
			if got != tt.want {
				t.Errorf("not_exists(%v) = %v, want %v", tt.attr, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_Matches(t *testing.T) {
	tests := []struct {
		name string
		attr any
		cond any
		want bool
	}{
		{"simple match", "hello123", `^hello\d+$`, true},
		{"no match", "world", `^hello\d+$`, false},
		{"email pattern", "user@example.com", `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, true},
		{"invalid regex", "hello", `[invalid`, false},
		{"empty string matches empty pattern", "", `^$`, true},
		{"partial match", "hello world", `world`, true},
		{"number as string", 12345, `^\d+$`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.attr, "matches", tt.cond)
			if got != tt.want {
				t.Errorf("matches(%v, %v) = %v, want %v", tt.attr, tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_UnknownOperator(t *testing.T) {
	got := EvaluateCondition("hello", "unknown_op", "hello")
	if got != false {
		t.Errorf("unknown operator should return false, got %v", got)
	}
}
