package netutil

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckGitHubConnectivity_Success(t *testing.T) {
	var userAgent atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent.Store(r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cleanup := SetConnectivityURLForTesting(server.URL)
	defer cleanup()

	connected := CheckGitHubConnectivity(500 * time.Millisecond)
	if !connected {
		t.Fatalf("expected connectivity to be true, got false")
	}

	ua, _ := userAgent.Load().(string)
	if ua != "repokit-connectivity-probe" {
		t.Errorf("expected User-Agent 'repokit-connectivity-probe', got '%s'", ua)
	}
}

func TestCheckGitHubConnectivity_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"200 OK", http.StatusOK, true},
		{"403 Forbidden (e.g. rate limit)", http.StatusForbidden, true},
		{"404 Not Found", http.StatusNotFound, true},
		{"500 Internal Server Error", http.StatusInternalServerError, false},
		{"502 Bad Gateway", http.StatusBadGateway, false},
		{"503 Service Unavailable", http.StatusServiceUnavailable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cleanup := SetConnectivityURLForTesting(server.URL)
			defer cleanup()

			got := CheckGitHubConnectivity(500 * time.Millisecond)
			if got != tt.expected {
				t.Errorf("status %d: expected %v, got %v", tt.statusCode, tt.expected, got)
			}
		})
	}
}

func TestCheckGitHubConnectivity_HeadFailure_GetFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// Simulate server rejecting HEAD with 500 or closing hijack
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cleanup := SetConnectivityURLForTesting(server.URL)
	defer cleanup()

	got := CheckGitHubConnectivity(500 * time.Millisecond)
	if !got {
		t.Errorf("expected connectivity to succeed via GET fallback, got false")
	}
}

func TestCheckGitHubConnectivity_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cleanup := SetConnectivityURLForTesting(server.URL)
	defer cleanup()

	// Short timeout should expire
	got := CheckGitHubConnectivity(50 * time.Millisecond)
	if got {
		t.Errorf("expected connectivity to time out and return false, got true")
	}
}

func TestCheckGitHubConnectivity_ClosedServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := server.URL
	server.Close() // Close immediately

	cleanup := SetConnectivityURLForTesting(closedURL)
	defer cleanup()

	got := CheckGitHubConnectivity(200 * time.Millisecond)
	if got {
		t.Errorf("expected connectivity to closed server to return false, got true")
	}
}

func TestCheckGitHubConnectivity_DefaultTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cleanup := SetConnectivityURLForTesting(server.URL)
	defer cleanup()

	// 0 or negative timeout should use default 1500ms
	got := CheckGitHubConnectivity(0)
	if !got {
		t.Errorf("expected connectivity with timeout=0 to succeed, got false")
	}
}

func TestIsOfflineMode(t *testing.T) {
	tests := []struct {
		envVal   string
		expected bool
	}{
		{"", false},
		{"0", false},
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"false", false},
		{"FALSE", false},
		{"yes", false},
	}

	for _, tt := range tests {
		t.Run("env="+tt.envVal, func(t *testing.T) {
			t.Setenv("REPOKIT_OFFLINE", tt.envVal)
			got := IsOfflineMode()
			if got != tt.expected {
				t.Errorf("REPOKIT_OFFLINE=%q: expected %v, got %v", tt.envVal, tt.expected, got)
			}
		})
	}
}

func TestSetConnectivityURLForTesting_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			cleanup := SetConnectivityURLForTesting(server.URL)
			_ = GetConnectivityURL()
			cleanup()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = GetConnectivityURL()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
}
