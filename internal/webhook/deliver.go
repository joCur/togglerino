package webhook

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

var deliveryTransport http.RoundTripper = &http.Transport{
	DialContext: ssrfSafeDialer,
}

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

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: deliveryTransport,
	}
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

func GenerateDeliveryID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func ssrfSafeDialer(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("dns lookup failed: %w", err)
	}

	for _, ip := range ips {
		if isPrivateIP(ip.IP) {
			return nil, fmt.Errorf("resolved to private IP address %s", ip.IP)
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}
