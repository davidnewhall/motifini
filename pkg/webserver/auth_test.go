package webserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKeyValid(t *testing.T) {
	t.Parallel()

	if !apiKeyValid("secret", "secret") {
		t.Fatal("matching keys must pass")
	}

	if apiKeyValid("Secret", "secret") || apiKeyValid("secret", "") || apiKeyValid("", "secret") {
		t.Fatal("mismatched or empty keys must fail")
	}

	if apiKeyValid("short", "a much longer key") {
		t.Fatal("different-length keys must fail")
	}
}

func TestExtractAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		header map[string]string
		want   string
	}{
		{"bearer", "/", map[string]string{"Authorization": "Bearer abc123"}, "abc123"},
		{"bearer case", "/", map[string]string{"Authorization": "bearer abc123"}, "abc123"},
		{"x-api-key", "/", map[string]string{"X-API-Key": "abc123"}, "abc123"},
		{"query", "/?apikey=abc123", nil, "abc123"},
		{"bearer wins", "/?apikey=wrong", map[string]string{
			"Authorization": "Bearer abc123", "X-API-Key": "also-wrong",
		}, "abc123"},
		{"basic ignored", "/", map[string]string{"Authorization": "Basic dXNlcg=="}, ""},
		{"none", "/", nil, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, test.target, nil)
			for k, v := range test.header {
				req.Header.Set(k, v)
			}

			if got := extractAPIKey(req); got != test.want {
				t.Fatalf("extractAPIKey: got %q want %q", got, test.want)
			}
		})
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{"127.0.0.1", "127.0.1.2", "::1", "localhost", "LOCALHOST"} {
		if !isLoopbackAddr(addr) {
			t.Fatalf("%s should be loopback", addr)
		}
	}

	for _, addr := range []string{"0.0.0.0", "::", "192.168.1.5", "myhost.local", "not-an-ip"} {
		if isLoopbackAddr(addr) {
			t.Fatalf("%s should not be loopback", addr)
		}
	}
}

func TestRequireAPIKey(t *testing.T) {
	t.Parallel()

	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		configured string
		target     string
		header     map[string]string
		want       int
	}{
		{"no key configured", "", "/x", nil, http.StatusOK},
		{"missing key", "k1", "/x", nil, http.StatusUnauthorized},
		{"wrong key", "k1", "/x", map[string]string{"X-API-Key": "nope"}, http.StatusUnauthorized},
		{"bearer", "k1", "/x", map[string]string{"Authorization": "Bearer k1"}, http.StatusOK},
		{"header", "k1", "/x", map[string]string{"X-API-Key": "k1"}, http.StatusOK},
		{"query", "k1", "/x?apikey=k1", nil, http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg, _ := testConfig(t)
			cfg.APIKey = test.configured

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, test.target, nil)
			for k, v := range test.header {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			cfg.requireAPIKey(okHandler).ServeHTTP(rec, req)

			if rec.Code != test.want {
				t.Fatalf("%s: code=%d want %d", test.name, rec.Code, test.want)
			}
		})
	}
}

// TestAPIKeyEndToEnd serves the real handler stack with a key configured and
// checks that every route — including /debug/vars — requires it.
func TestAPIKeyEndToEnd(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfig(t)
	cfg.APIKey = "end-to-end-key"

	server := httptest.NewServer(cfg.handler())
	t.Cleanup(server.Close)

	if code := getWithKey(t, server.URL+"/api/v1.0/events", ""); code != http.StatusUnauthorized {
		t.Fatalf("events without key: got %d want 401", code)
	}

	if code := getWithKey(t, server.URL+"/debug/vars", ""); code != http.StatusUnauthorized {
		t.Fatalf("debug/vars without key: got %d want 401", code)
	}

	if code := getWithKey(t, server.URL+"/api/v1.0/events", "wrong"); code != http.StatusUnauthorized {
		t.Fatalf("events with wrong key: got %d want 401", code)
	}

	if code := getWithKey(t, server.URL+"/api/v1.0/events", "end-to-end-key"); code != http.StatusOK {
		t.Fatalf("events with key: got %d want 200", code)
	}

	if code := getWithKey(t, server.URL+"/api/v1.0/events?apikey=end-to-end-key", ""); code != http.StatusOK {
		t.Fatalf("events with query key: got %d want 200", code)
	}
}

func getWithKey(t *testing.T, url, key string) int {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}

	defer resp.Body.Close()

	return resp.StatusCode
}
