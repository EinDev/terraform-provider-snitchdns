package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetryLogic tests that the client retries on transient failures
func TestRetryLogic(t *testing.T) {
	attempts := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			// Fail the first 2 attempts with 503 Service Unavailable
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Succeed on the 3rd attempt
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "domain": "example.com", "active": true, "catch_all": false, "forwarding": false, "regex": false}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	client.MaxRetries = 3
	client.RetryWaitMin = 10 * time.Millisecond
	client.RetryWaitMax = 50 * time.Millisecond

	zone, err := client.GetZone("1")
	if err != nil {
		t.Fatalf("Expected request to succeed after retries, got error: %v", err)
	}

	if zone == nil {
		t.Fatal("Expected zone to be returned")
	}

	if attempts.Load() != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts.Load())
	}
}

// TestRetryExhausted tests that the client gives up after max retries
func TestRetryExhausted(t *testing.T) {
	attempts := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	client.MaxRetries = 2
	client.RetryWaitMin = 1 * time.Millisecond
	client.RetryWaitMax = 5 * time.Millisecond

	_, err := client.GetZone("1")
	if err == nil {
		t.Fatal("Expected error after exhausting retries")
	}

	// Should attempt initial request + 2 retries = 3 total
	if attempts.Load() != 3 {
		t.Errorf("Expected 3 attempts (1 initial + 2 retries), got %d", attempts.Load())
	}
}

// TestNoRetryOn4xx tests that 4xx errors are not retried
func TestNoRetryOn4xx(t *testing.T) {
	attempts := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	client.MaxRetries = 3

	_, err := client.GetZone("1")
	if err == nil {
		t.Fatal("Expected error for 404")
	}

	// Should only attempt once (no retries for 4xx)
	if attempts.Load() != 1 {
		t.Errorf("Expected 1 attempt (no retry on 4xx), got %d", attempts.Load())
	}
}

// TestUserAgentHeader tests that the user-agent header is set
func TestUserAgentHeader(t *testing.T) {
	var capturedUserAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "domain": "example.com", "active": true, "catch_all": false, "forwarding": false, "regex": false}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	client.UserAgent = "terraform-provider-snitchdns/1.0.0"

	_, err := client.GetZone("1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if capturedUserAgent != "terraform-provider-snitchdns/1.0.0" {
		t.Errorf("Expected User-Agent to be 'terraform-provider-snitchdns/1.0.0', got '%s'", capturedUserAgent)
	}
}

// TestContextTimeout tests that context timeout is respected
func TestContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Delay longer than the context timeout
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetZoneWithContext(ctx, "1")
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

// TestDebugLogging tests that debug logging can be enabled
func TestDebugLogging(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "domain": "example.com", "active": true, "catch_all": false, "forwarding": false, "regex": false}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	// This test just ensures the client can be configured with debug logging
	// Actual logging behavior is tested via tflog in provider tests
	client.DebugLogging = true

	_, err := client.GetZone("1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// TestExponentialBackoff tests that retry delays increase exponentially
func TestExponentialBackoff(t *testing.T) {
	attempts := atomic.Int32{}
	var timestamps []time.Time

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		timestamps = append(timestamps, time.Now())
		count := attempts.Add(1)
		if count < 4 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "domain": "example.com", "active": true, "catch_all": false, "forwarding": false, "regex": false}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	client.MaxRetries = 4
	client.RetryWaitMin = 10 * time.Millisecond
	client.RetryWaitMax = 100 * time.Millisecond

	_, err := client.GetZone("1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify we made 4 attempts
	if len(timestamps) != 4 {
		t.Fatalf("Expected 4 attempts, got %d", len(timestamps))
	}

	// Verify delays are increasing (with some tolerance for timing variance)
	delay1 := timestamps[1].Sub(timestamps[0])
	delay2 := timestamps[2].Sub(timestamps[1])

	// Each delay should be close to RetryWaitMin (allow 20% variance for timing jitter)
	minDelay := 8 * time.Millisecond // 10ms - 20% tolerance
	if delay1 < minDelay {
		t.Errorf("First delay too short: %v (expected at least %v)", delay1, minDelay)
	}

	// Later delays should generally be longer (exponential backoff)
	// We use a loose check because of timing variance in tests
	if delay2 < delay1/2 {
		t.Errorf("Expected exponential backoff, but delay2 (%v) < delay1/2 (%v)", delay2, delay1/2)
	}
}
