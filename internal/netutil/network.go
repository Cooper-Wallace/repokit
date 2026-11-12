package netutil

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	connectivityMu  sync.RWMutex
	connectivityURL = "https://api.github.com"
)

// GetConnectivityURL returns the target URL for connectivity checks.
func GetConnectivityURL() string {
	connectivityMu.RLock()
	defer connectivityMu.RUnlock()
	return connectivityURL
}

// SetConnectivityURLForTesting temporarily overrides the connectivity URL for testing and returns a cleanup func.
func SetConnectivityURLForTesting(urlStr string) func() {
	connectivityMu.Lock()
	old := connectivityURL
	connectivityURL = urlStr
	connectivityMu.Unlock()
	return func() {
		connectivityMu.Lock()
		connectivityURL = old
		connectivityMu.Unlock()
	}
}

// setConnectivityURL provides package-internal backward compatibility for test helpers.
func setConnectivityURL(urlStr string) func() {
	return SetConnectivityURLForTesting(urlStr)
}

// CheckGitHubConnectivity probes GitHub connectivity using standard library net/http.
// If timeout <= 0, it defaults to 1500ms.
// It sends a HEAD request (with GET fallback) to https://api.github.com with
// User-Agent: repokit-connectivity-probe.
// Returns true if response is received with status code < 500.
// Returns false on network errors, DNS errors, connection refusal, timeout, or status code >= 500.
func CheckGitHubConnectivity(timeout time.Duration) bool {
	return checkConnectivityEndpoint(GetConnectivityURL(), timeout)
}

func checkConnectivityEndpoint(targetURL string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	client := &http.Client{
		Timeout: timeout,
	}

	// 1. Try HEAD request first
	headReq, err := http.NewRequest(http.MethodHead, targetURL, nil)
	if err == nil {
		headReq.Header.Set("User-Agent", "repokit-connectivity-probe")
		resp, err := client.Do(headReq)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return true
			}
		}
	}

	// 2. Fallback to GET request
	getReq, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return false
	}
	getReq.Header.Set("User-Agent", "repokit-connectivity-probe")

	resp, err := client.Do(getReq)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}

// IsOfflineMode checks if environment variable REPOKIT_OFFLINE is set to "1" or "true" (case-insensitive).
func IsOfflineMode() bool {
	val := strings.TrimSpace(os.Getenv("REPOKIT_OFFLINE"))
	return val == "1" || strings.EqualFold(val, "true")
}
