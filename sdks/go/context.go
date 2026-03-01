package togglerino

import "context"

// GetContext returns a copy of the current evaluation context.
// Modifying the returned value does not affect the client's state.
func (c *Client) GetContext() EvaluationContext {
	c.flagsMu.RLock()
	defer c.flagsMu.RUnlock()
	ctx := c.config.context
	if ctx.Attributes != nil {
		attrs := make(map[string]any, len(ctx.Attributes))
		for k, v := range ctx.Attributes {
			attrs[k] = v
		}
		ctx.Attributes = attrs
	}
	return ctx
}

// UpdateContext merges the provided evaluation context into the client's
// current context (non-empty UserID replaces, attributes are merged),
// then re-fetches all flags and emits a context_change event.
func (c *Client) UpdateContext(ctx context.Context, evalCtx *EvaluationContext) error {
	c.flagsMu.Lock()
	if evalCtx.UserID != "" {
		c.config.context.UserID = evalCtx.UserID
	}
	if evalCtx.Attributes != nil {
		if c.config.context.Attributes == nil {
			c.config.context.Attributes = make(map[string]any)
		}
		for k, v := range evalCtx.Attributes {
			c.config.context.Attributes[k] = v
		}
	}
	c.flagsMu.Unlock()

	if err := c.fetchFlags(ctx); err != nil {
		return err
	}
	c.events.emit(eventContextChange, c.GetContext())
	return nil
}
