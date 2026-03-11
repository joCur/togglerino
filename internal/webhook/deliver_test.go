package webhook

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func TestMain(m *testing.M) {
	deliveryTransport = http.DefaultTransport
	os.Exit(m.Run())
}

func TestDeliver_Success(t *testing.T) {
	var called atomic.Bool
	var gotSig, gotEvent, gotDeliveryID string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		gotSig = r.Header.Get("X-Togglerino-Signature")
		gotEvent = r.Header.Get("X-Togglerino-Event")
		gotDeliveryID = r.Header.Get("X-Togglerino-Delivery")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	result := Deliver(ts.URL, "whsec_test", "flag.created", "delivery-123", []byte(`{"type":"flag.created"}`))

	if !called.Load() {
		t.Fatal("endpoint was not called")
	}
	if !result.Success {
		t.Errorf("Success = false, want true; error: %v", result.Error)
	}
	if result.StatusCode == nil || *result.StatusCode != 200 {
		t.Errorf("StatusCode = %v, want 200", result.StatusCode)
	}
	if gotSig == "" {
		t.Error("X-Togglerino-Signature header missing")
	}
	if gotEvent != "flag.created" {
		t.Errorf("X-Togglerino-Event = %q, want flag.created", gotEvent)
	}
	if gotDeliveryID != "delivery-123" {
		t.Errorf("X-Togglerino-Delivery = %q, want delivery-123", gotDeliveryID)
	}
}

func TestDeliver_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer ts.Close()

	result := Deliver(ts.URL, "whsec_test", "flag.created", "d-1", []byte(`{}`))
	if result.Success {
		t.Error("Success = true, want false for 500 response")
	}
	if result.StatusCode == nil || *result.StatusCode != 500 {
		t.Errorf("StatusCode = %v, want 500", result.StatusCode)
	}
}

func TestDeliver_SignatureIsCorrect(t *testing.T) {
	payload := []byte(`{"type":"test"}`)
	secret := "whsec_mysecret"
	var gotSig string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Togglerino-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	Deliver(ts.URL, secret, "test", "d-1", payload)
	expected := "sha256=" + Sign(payload, secret)
	if gotSig != expected {
		t.Errorf("signature = %q, want %q", gotSig, expected)
	}
}

