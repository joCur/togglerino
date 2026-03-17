package togglerino

import "encoding/json"

// Evaluator holds the results of evaluating all flags for a specific
// EvaluationContext. It is returned by Server.Evaluate and is safe to
// use concurrently — however, it is a snapshot and will not update when
// the server's cached definitions change.
type Evaluator struct {
	results map[string]EvaluationResult
}

// BoolValue returns the boolean value of the named flag, or defaultValue
// if the flag is missing or not a boolean.
func (e *Evaluator) BoolValue(key string, defaultValue bool) bool {
	result, ok := e.results[key]
	if !ok {
		return defaultValue
	}
	v, ok := result.Value.(bool)
	if !ok {
		return defaultValue
	}
	return v
}

// StringValue returns the string value of the named flag, or defaultValue
// if the flag is missing or not a string.
func (e *Evaluator) StringValue(key string, defaultValue string) string {
	result, ok := e.results[key]
	if !ok {
		return defaultValue
	}
	v, ok := result.Value.(string)
	if !ok {
		return defaultValue
	}
	return v
}

// NumberValue returns the float64 value of the named flag, or defaultValue
// if the flag is missing or not a number.
func (e *Evaluator) NumberValue(key string, defaultValue float64) float64 {
	result, ok := e.results[key]
	if !ok {
		return defaultValue
	}
	v, ok := result.Value.(float64)
	if !ok {
		return defaultValue
	}
	return v
}

// JSONValue unmarshals the named flag's value into target. If the flag is
// missing, defaultValue is used instead. Returns an error if marshaling
// or unmarshaling fails.
func (e *Evaluator) JSONValue(key string, target any, defaultValue any) error {
	var src any
	if result, ok := e.results[key]; ok {
		src = result.Value
	} else {
		src = defaultValue
	}

	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Detail returns the full EvaluationResult for a flag. The second return
// value is false if the flag does not exist in the evaluator's results.
func (e *Evaluator) Detail(key string) (EvaluationResult, bool) {
	result, ok := e.results[key]
	if !ok {
		return EvaluationResult{}, false
	}
	return result, true
}
