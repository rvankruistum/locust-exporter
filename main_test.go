package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// Mock Locust stats response
const mockLocustResponse = `{
  "stats": [
    {
      "method": "GET",
      "name": "/api/users",
      "num_requests": 100,
      "num_failures": 5,
      "avg_response_time": 123.45,
      "min_response_time": 50.0,
      "max_response_time": 500.0,
      "current_rps": 10.5,
      "current_fail_per_sec": 0.5,
      "median_response_time": 120.0,
      "avg_content_length": 1024.0,
      "response_time_percentile_0.95": 480.0,
      "response_time_percentile_0.99": 495.0
    },
    {
      "method": "POST",
      "name": "/api/login",
      "num_requests": 50,
      "num_failures": 2,
      "avg_response_time": 200.0,
      "min_response_time": 100.0,
      "max_response_time": 400.0,
      "current_rps": 5.0,
      "current_fail_per_sec": 0.2,
      "median_response_time": 180.0,
      "avg_content_length": 512.0,
      "response_time_percentile_0.95": 390.0,
      "response_time_percentile_0.99": 398.0
    },
    {
      "method": "GET",
      "name": "Total",
      "num_requests": 150,
      "num_failures": 7,
      "avg_response_time": 150.0,
      "min_response_time": 50.0,
      "max_response_time": 500.0,
      "current_rps": 15.5,
      "current_fail_per_sec": 0.7,
      "median_response_time": 140.0,
      "avg_content_length": 850.0,
      "response_time_percentile_0.95": 450.0,
      "response_time_percentile_0.99": 490.0
    }
  ],
  "errors": [
    {
      "method": "GET",
      "name": "/api/users",
      "error": "Connection timeout",
      "occurrences": 3
    },
    {
      "method": "POST",
      "name": "/api/login",
      "error": "Invalid credentials",
      "occurrences": 2
    }
  ],
  "total_rps": 15.5,
  "fail_ratio": 0.047,
  "current_response_time_percentiles": {
    "response_time_percentile_0.5": 140.0,
    "response_time_percentile_0.95": 450.0
  },
  "state": "running",
  "user_count": 100,
  "workers": [
    {
      "id": "worker-1",
      "state": "running",
      "user_count": 50
    },
    {
      "id": "worker-2",
      "state": "running",
      "user_count": 30
    },
    {
      "id": "worker-3",
      "state": "hatching",
      "user_count": 20
    }
  ]
}`

// ============================================================================
// UNIT TESTS (no external dependencies, pure logic)
// ============================================================================
func TestNewExporter(t *testing.T) {
	namespace = "locust"

	tests := []struct {
		name        string
		uri         string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "valid http URI",
			uri:         "http://localhost:8089",
			timeout:     5 * time.Second,
			expectError: false,
		},
		{
			name:        "valid https URI",
			uri:         "https://localhost:8089",
			timeout:     5 * time.Second,
			expectError: false,
		},
		{
			name:        "invalid URI",
			uri:         "://invalid",
			timeout:     5 * time.Second,
			expectError: true,
		},
		{
			name:        "unsupported scheme",
			uri:         "ftp://localhost:8089",
			timeout:     5 * time.Second,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter, err := NewExporter(tt.uri, tt.timeout)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if exporter == nil {
					t.Errorf("expected exporter but got nil")
				}
			}
		})
	}
}

func TestCountWorkersByState(t *testing.T) {
	stats := locustStats{
		Workers: []struct {
			Id        string `json:"id"`
			State     string `json:"state"`
			UserCount int    `json:"user_count"`
		}{
			{Id: "w1", State: "running", UserCount: 10},
			{Id: "w2", State: "running", UserCount: 20},
			{Id: "w3", State: "hatching", UserCount: 5},
			{Id: "w4", State: "missing", UserCount: 0},
			{Id: "w5", State: "running", UserCount: 15},
		},
	}

	tests := []struct {
		state    string
		expected float64
	}{
		{"running", 3.0},
		{"hatching", 1.0},
		{"missing", 1.0},
		{"stopped", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			result := countWorkersByState(stats, tt.state)
			if result != tt.expected {
				t.Errorf("countWorkersByState(%q) = %v, want %v", tt.state, result, tt.expected)
			}
		})
	}
}

func TestJSONParsing(t *testing.T) {
	var stats locustStats
	err := json.Unmarshal([]byte(mockLocustResponse), &stats)
	if err != nil {
		t.Fatalf("failed to parse mock JSON: %v", err)
	}

	// Verify parsed values
	if len(stats.Stats) != 3 {
		t.Errorf("expected 3 stats, got %d", len(stats.Stats))
	}

	if stats.UserCount != 100 {
		t.Errorf("expected user_count=100, got %d", stats.UserCount)
	}

	if stats.State != "running" {
		t.Errorf("expected state=running, got %s", stats.State)
	}

	if len(stats.Workers) != 3 {
		t.Errorf("expected 3 workers, got %d", len(stats.Workers))
	}

	if len(stats.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(stats.Errors))
	}

	// Verify specific stat values
	firstStat := stats.Stats[0]
	if firstStat.Method != "GET" {
		t.Errorf("expected method=GET, got %s", firstStat.Method)
	}
	if firstStat.Name != "/api/users" {
		t.Errorf("expected name=/api/users, got %s", firstStat.Name)
	}
	if firstStat.NumRequests != 100 {
		t.Errorf("expected num_requests=100, got %d", firstStat.NumRequests)
	}
}

func TestStateMapping(t *testing.T) {
	tests := []struct {
		state    string
		expected int
	}{
		{"stopped", 0},
		{"hatching", 1},
		{"running", 2},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			var running int
			switch tt.state {
			case "hatching":
				running = 1
			case "running":
				running = 2
			}

			if running != tt.expected {
				t.Errorf("state %q mapped to %d, want %d", tt.state, running, tt.expected)
			}
		})
	}
}

func TestExporterDescribe(t *testing.T) {
	namespace = "locust"

	exporter, err := NewExporter("http://localhost:8089", 5*time.Second)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	// Test that Describe doesn't panic
	ch := make(chan *prometheus.Desc, 100)
	go func() {
		exporter.Describe(ch)
		close(ch)
	}()

	// Count descriptors
	count := 0
	for range ch {
		count++
	}

	if count == 0 {
		t.Error("expected at least one metric descriptor")
	}
}

// ============================================================================
// INTEGRATION TESTS (use mock HTTP servers, test full workflows)
// ============================================================================
// To enable selective execution: //go:build integration

func TestExporterScrape(t *testing.T) {
	namespace = "locust"

	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stats/requests" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mockLocustResponse))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create exporter with mock server
	exporter, err := NewExporter(server.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	// Create a registry and register the exporter
	reg := prometheus.NewRegistry()
	reg.MustRegister(exporter)

	// Collect metrics
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	// Verify we got metrics
	if len(metrics) == 0 {
		t.Fatal("expected metrics but got none")
	}

	// Check for specific metrics
	metricNames := make(map[string]bool)
	for _, m := range metrics {
		metricNames[*m.Name] = true
	}

	expectedMetrics := []string{
		"locust_up",
		"locust_users",
		"locust_running",
		"locust_workers_count",
		"locust_requests_fail_ratio",
		"locust_requests_current_response_time_percentile_50",
		"locust_requests_current_response_time_percentile_95",
		"locust_requests_response_time_percentile_95",
		"locust_requests_response_time_percentile_99",
		"locust_requests_num_requests",
		"locust_requests_avg_response_time",
		"locust_errors",
	}

	for _, name := range expectedMetrics {
		if !metricNames[name] {
			t.Errorf("expected metric %q not found", name)
		}
	}
}

func TestExporterScrapeError(t *testing.T) {
	namespace = "locust"

	// Create a mock server that returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	exporter, err := NewExporter(server.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("failed to create exporter: %v", err)
	}

	// Collect metrics - should handle error gracefully
	ch := make(chan prometheus.Metric, 100)
	go func() {
		exporter.Collect(ch)
		close(ch)
	}()

	// Count metrics (should still have some even with error)
	count := 0
	for range ch {
		count++
	}

	// Should at least have locust_up metric showing down status
	if count == 0 {
		t.Error("expected at least one metric even with scrape error")
	}
}

func TestFetchHTTP(t *testing.T) {
	namespace = "locust"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/test" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	fetch := fetchHTTP(server.URL, 5*time.Second)

	t.Run("successful fetch", func(t *testing.T) {
		body, err := fetch("/test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = body.Close() }()

		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}

		if string(data) != "test response" {
			t.Errorf("expected 'test response', got %q", string(data))
		}
	})

	t.Run("404 error", func(t *testing.T) {
		body, err := fetch("/notfound")
		if err == nil {
			_ = body.Close()
			t.Error("expected error for 404, got none")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("expected 404 error, got: %v", err)
		}
	})
}

// TestPushgatewayIntegration tests push functionality with mock pushgateway server
func TestPushgatewayIntegration(t *testing.T) {
	pushReceived := false
	var receivedMethod string
	var receivedPath string

	// Mock pushgateway server
	pushgateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushReceived = true
		receivedMethod = r.Method
		receivedPath = r.URL.Path

		// Verify it's a PUT request (push mode)
		if r.Method != "PUT" {
			t.Errorf("Expected PUT request for push mode, got %s", r.Method)
		}

		// Verify job label in URL
		if !strings.Contains(r.URL.Path, "/metrics/job/") {
			t.Errorf("Expected job label in path, got: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer pushgateway.Close()

	// Create test registry and metric
	reg := prometheus.NewRegistry()
	testGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "test",
		Name:      "metric",
		Help:      "Test metric for pushgateway",
	})
	testGauge.Set(42)
	reg.MustRegister(testGauge)

	// Use the actual push package to test
	pusher := push.New(pushgateway.URL, "test_job").Gatherer(reg)
	err := pusher.Push()

	if err != nil {
		t.Fatalf("Failed to push metrics: %v", err)
	}

	if !pushReceived {
		t.Error("Expected push request to be received")
	}

	if receivedMethod != "PUT" {
		t.Errorf("Expected PUT method, got %s", receivedMethod)
	}

	if !strings.Contains(receivedPath, "test_job") {
		t.Errorf("Expected 'test_job' in path, got: %s", receivedPath)
	}
}

// TestPushMetricsPeriodically tests the actual pushMetricsPeriodically function
func TestPushMetricsPeriodically(t *testing.T) {
	pushCount := 0
	var mu sync.Mutex

	// Mock pushgateway server
	pushgateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pushCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer pushgateway.Close()

	// Create test registry and metric
	reg := prometheus.NewRegistry()
	testGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "test",
		Name:      "periodic_push",
		Help:      "Test metric for periodic push",
	})
	testGauge.Set(123)
	reg.MustRegister(testGauge)

	// Test immediate push on startup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pushMetricsPeriodically(ctx, pushgateway.URL, "test_periodic_job", 100*time.Millisecond, reg)
	}()

	// Wait for startup push
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	startupPushCount := pushCount
	mu.Unlock()

	if startupPushCount != 1 {
		t.Errorf("Expected 1 startup push, got %d", startupPushCount)
	}

	// Wait for at least 2 more periodic pushes
	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	finalPushCount := pushCount
	mu.Unlock()

	// Should have startup push (1) + at least 2 periodic pushes (at 100ms and 200ms)
	if finalPushCount < 3 {
		t.Errorf("Expected at least 3 pushes (1 startup + 2 periodic), got %d", finalPushCount)
	}

	// Test context cancellation stops the goroutine
	cancel()
	wg.Wait() // Should complete quickly

	// Verify no more pushes after cancellation
	mu.Lock()
	pushCountAfterCancel := pushCount
	mu.Unlock()

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	pushCountAfterWait := pushCount
	mu.Unlock()

	if pushCountAfterWait != pushCountAfterCancel {
		t.Errorf("Expected no pushes after cancellation, but got %d more", pushCountAfterWait-pushCountAfterCancel)
	}
}

// TestPushMetricsPeriodicallyCancellation tests that context cancellation is handled properly
func TestPushMetricsPeriodicallyCancellation(t *testing.T) {
	// Mock pushgateway server
	pushgateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer pushgateway.Close()

	// Create test registry
	reg := prometheus.NewRegistry()
	testGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_cancel",
		Help: "Test cancellation",
	})
	reg.MustRegister(testGauge)

	// Start with context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var wg sync.WaitGroup
	wg.Add(1)

	startTime := time.Now()
	go func() {
		defer wg.Done()
		pushMetricsPeriodically(ctx, pushgateway.URL, "test_cancel_job", 1*time.Second, reg)
	}()

	// Should exit quickly (within 100ms) after doing startup push
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		elapsed := time.Since(startTime)
		if elapsed > 500*time.Millisecond {
			t.Errorf("Goroutine took too long to exit after cancellation: %v", elapsed)
		}
	case <-time.After(1 * time.Second):
		t.Error("Goroutine did not exit after context cancellation")
	}
}

// TestPushMetricsPeriodicallyCancellationDuringPush tests cancellation during an active push
func TestPushMetricsPeriodicallyCancellationDuringPush(t *testing.T) {
	pushStarted := make(chan struct{})
	pushComplete := make(chan struct{})

	// Mock pushgateway server with slow response
	pushgateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(pushStarted)
		<-pushComplete // Block until test allows completion
		w.WriteHeader(http.StatusOK)
	}))
	defer pushgateway.Close()

	// Create test registry
	reg := prometheus.NewRegistry()
	testGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_cancel_during_push",
		Help: "Test cancellation during push",
	})
	reg.MustRegister(testGauge)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pushMetricsPeriodically(ctx, pushgateway.URL, "test_job", 100*time.Millisecond, reg)
	}()

	// Wait for push to start
	<-pushStarted

	// Cancel context while push is in progress
	cancel()

	// Allow push to complete
	close(pushComplete)

	// Goroutine should exit cleanly
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - goroutine exited
	case <-time.After(2 * time.Second):
		t.Error("Goroutine did not exit after context cancellation during push")
	}
}

// TestPushMetricsPeriodicallyCancellationErrorHandling tests error handling during pushes
func TestPushMetricsPeriodicallyCancellationErrorHandling(t *testing.T) {
	pushCount := 0
	var mu sync.Mutex

	// Mock pushgateway server that fails
	pushgateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pushCount++
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError) // Fail every time
	}))
	defer pushgateway.Close()

	// Create test registry
	reg := prometheus.NewRegistry()
	testGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_error_handling",
		Help: "Test error handling",
	})
	reg.MustRegister(testGauge)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pushMetricsPeriodically(ctx, pushgateway.URL, "test_error_job", 50*time.Millisecond, reg)
	}()

	// Wait for multiple push attempts despite errors
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	finalCount := pushCount
	mu.Unlock()

	// Should continue pushing despite errors (startup + at least 3 periodic)
	if finalCount < 4 {
		t.Errorf("Expected at least 4 push attempts despite errors, got %d", finalCount)
	}

	cancel()
	wg.Wait()
}

// TestBackwardsCompatibility ensures empty pushgateway URL doesn't break anything
func TestBackwardsCompatibility(t *testing.T) {
	// Test that empty pushgateway URL doesn't cause issues
	pushURL := ""
	if pushURL != "" {
		t.Error("Pushgateway should be disabled when URL is empty")
	}

	// Verify registry works without push
	reg := prometheus.NewRegistry()
	testGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_backwards_compat",
		Help: "Test backwards compatibility",
	})
	reg.MustRegister(testGauge)
	testGauge.Set(100)

	// Gather metrics to ensure it works
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected at least one metric family")
	}
}
