package webserver

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/davidnewhall/motifini/pkg/messenger"
)

// apiKeyParam is the query parameter carrying the key for callers that cannot
// set headers.
const apiKeyParam = "apikey"

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

	return request.URL.Query().Get(apiKeyParam)
}

// apiKeyValid compares keys in constant time. Both sides are hashed first so
// the comparison does not leak the configured key's length.
func apiKeyValid(got, want string) bool {
	gotSum := sha256.Sum256([]byte(got))
	wantSum := sha256.Sum256([]byte(want))

	return subtle.ConstantTimeCompare(gotSum[:], wantSum[:]) == 1
}

// redactAPIKey returns the request URL with every apikey query value replaced,
// so query-string credentials never land in the application log.
//
// Redaction is deliberately broader than extraction: it ignores case and covers
// repeated parameters. A key that authentication rejects — `?apikey=&apikey=x`,
// where Get returns only the empty first value, or a miscased `?APIKEY=x` — is
// still a real credential, and the 401 gets logged the same as any other reply.
func redactAPIKey(request *http.Request) string {
	query := request.URL.Query()
	redacted := false

	for param := range query {
		if strings.EqualFold(param, apiKeyParam) {
			query.Set(param, "REDACTED") // Set drops any repeated values.

			redacted = true
		}
	}

	if !redacted {
		return request.URL.String()
	}

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
