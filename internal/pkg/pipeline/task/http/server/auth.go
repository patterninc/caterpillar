package server

import (
	"encoding/base64"
	"net"
	"net/http"
	"strings"
)

const (
	unauthorizedMessage = `{"ok":false, "error":"access denied"}`
	xForwardedFor       = `x-forwarded-for`
)

type authBehavior struct {
	Behavior     string            `yaml:"behavior,omitempty" json:"behavior,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	WhitelistIps []string          `yaml:"whitelist_ips,omitempty" json:"whitelist_ips,omitempty"`
	Username     string            `yaml:"username,omitempty" json:"username,omitempty"`
	Password     string            `yaml:"password,omitempty" json:"password,omitempty"`
}

func (s *server) apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for headerKey, expectedValue := range s.Auth.Headers {
			actualValue := r.Header.Get(headerKey)
			if actualValue != expectedValue {
				accessDeniedError(w)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) ipWhitelistMiddleware(next http.Handler) http.Handler {
	// Convert whitelist slice to map for O(1) lookup
	whitelistSet := make(map[string]struct{}, len(s.Auth.WhitelistIps))
	for _, ip := range s.Auth.WhitelistIps {
		whitelistSet[ip] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)

		// Check if the client IP is in the whitelist
		if _, allowed := whitelistSet[clientIP]; !allowed {
			accessDeniedError(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) basicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := extractBasicAuth(r)
		if !ok {
			accessDeniedError(w)
			return
		}

		// Check if credentials match configured username/password
		if username != s.Auth.Username || password != s.Auth.Password {
			accessDeniedError(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractBasicAuth extracts username and password from HTTP Basic Authentication
func extractBasicAuth(r *http.Request) (username, password string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", "", false
	}

	// Check if it's Basic auth
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return "", "", false
	}

	// Decode the base64 encoded credentials
	encoded := auth[len(prefix):]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", false
	}

	// Split username:password
	credentials := string(decoded)
	parts := strings.SplitN(credentials, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// getClientIP extracts the client IP address from the request
// Prioritizes X-Forwarded-For header for production environments with load balancers/proxies
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (most common in production)
	if xff := r.Header.Get(xForwardedFor); xff != "" {
		// X-Forwarded-For can contain multiple IPs separated by commas
		// The first IP is typically the original client IP
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// Fall back to RemoteAddr (direct connection)
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}

	// If all else fails, return RemoteAddr as-is
	return r.RemoteAddr
}

func accessDeniedError(w http.ResponseWriter) {
	w.Header().Set(contentTypeKey, contentTypeJson)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(unauthorizedMessage))
}
