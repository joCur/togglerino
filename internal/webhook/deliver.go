package webhook

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DeliveryResult struct {
	StatusCode   *int
	ResponseBody *string
	Error        *string
	Success      bool
	DurationMs   int
}

func Deliver(url, secret, eventType, deliveryID string, payload []byte) DeliveryResult {
	start := time.Now()

	sig := "sha256=" + Sign(payload, secret)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		errStr := fmt.Sprintf("creating request: %v", err)
		return DeliveryResult{Error: &errStr, DurationMs: int(time.Since(start).Milliseconds())}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Togglerino-Signature", sig)
	req.Header.Set("X-Togglerino-Event", eventType)
	req.Header.Set("X-Togglerino-Delivery", deliveryID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	durationMs := int(time.Since(start).Milliseconds())
	if err != nil {
		errStr := fmt.Sprintf("delivering webhook: %v", err)
		return DeliveryResult{Error: &errStr, DurationMs: durationMs}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	bodyStr := string(body)

	statusCode := resp.StatusCode
	success := statusCode >= 200 && statusCode < 300

	result := DeliveryResult{
		StatusCode:   &statusCode,
		ResponseBody: &bodyStr,
		Success:      success,
		DurationMs:   durationMs,
	}
	if !success {
		errStr := fmt.Sprintf("unexpected status code: %d", statusCode)
		result.Error = &errStr
	}
	return result
}
