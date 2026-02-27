package togglerino

import "encoding/json"

// BoolValue returns the boolean value of the named flag, or defaultValue
// if the flag is missing or not a boolean.
func (c *Client) BoolValue(key string, defaultValue bool) bool {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()
	result, ok := c.flags[key]
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
func (c *Client) StringValue(key string, defaultValue string) string {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()
	result, ok := c.flags[key]
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
func (c *Client) NumberValue(key string, defaultValue float64) float64 {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()
	result, ok := c.flags[key]
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
func (c *Client) JSONValue(key string, target any, defaultValue any) error {
	c.flagsMu.RLock()
	result, ok := c.flags[key]
	c.flagsMu.RUnlock()

	var src any
	if ok {
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
// value is false if the flag does not exist in the cache.
func (c *Client) Detail(key string) (EvaluationResult, bool) {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()
	result, ok := c.flags[key]
	if !ok {
		return EvaluationResult{}, false
	}
	return *result, true
}
