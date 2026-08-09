package webserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/davidnewhall/motifini/pkg/messenger"
)

// requireAPIKey wraps the router with API key authentication when a key is
// configured. With no key configured the API stays open (localhost default).
func (c *Config) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if c.APIKey == "" || apiKeyValid(extractAPIKey(request), c.APIKey) {
			next.ServeHTTP(writer, request)

			return
		}

		reqID := messenger.ReqID(messenger.IDLength)
		c.finishReq(writer, request, reqID, http.StatusUnauthorized,
			"ERROR: unauthorized (bad or missing api key)\n", "auth")
	})
}

// extractAPIKey pulls the key from the Authorization: Bearer header, the
// X-API-Key header, or the apikey query parameter (for webhook callers like
// SecuritySpy custom actions that cannot set headers).
func extractAPIKey(request *http.Request) string {
	if auth := request.Header.Get("Authorization"); auth != "" {
		if scheme, token, ok := strings.Cut(auth, " "); ok && strings.EqualFold(scheme, "bearer") {
			return strings.TrimSpace(token)
		}
	}

	if key := request.Header.Get("X-Api-Key"); key != "" {
		return key
	}

	return request.URL.Query().Get("apikey")
}

// apiKeyValid compares keys in constant time. Both sides are hashed first so
// the comparison does not leak the configured key's length.
func apiKeyValid(got, want string) bool {
	gotSum := sha256.Sum256([]byte(got))
	wantSum := sha256.Sum256([]byte(want))

	return subtle.ConstantTimeCompare(gotSum[:], wantSum[:]) == 1
}

// redactAPIKey returns the request URL with any apikey query value replaced,
// so query-string credentials never land in the application log.
func redactAPIKey(request *http.Request) string {
	query := request.URL.Query()
	if query.Get("apikey") == "" {
		return request.URL.String()
	}

	query.Set("apikey", "REDACTED")

	return request.URL.Path + "?" + query.Encode()
}

// isLoopbackAddr reports whether the listen address is localhost-only.
// Anything unparsable is treated as non-loopback (warns loudly at startup).
func isLoopbackAddr(addr string) bool {
	if strings.EqualFold(addr, "localhost") {
		return true
	}

	ip := net.ParseIP(addr)

	return ip != nil && ip.IsLoopback()
}
