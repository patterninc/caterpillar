package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtBearerGrantURN = `urn:ietf:params:oauth:grant-type:jwt-bearer`
	contentTypeJSON   = `application/json`
	authRefreshSkew   = 30 * time.Second
	authShortLifetime = 60 * time.Second
)

type authCache struct {
	mu       sync.Mutex
	entries  map[string]cachedToken
	inflight map[string]*tokenFetch
}

// tokenFetch is one in-flight token HTTP call. Waiters block on done.
type tokenFetch struct {
	done     chan struct{}
	token    string
	lifetime time.Duration
	err      error
}

type cachedToken struct {
	value     string
	expiresAt time.Time
	lifetime  time.Duration
}

func tokenUsable(now time.Time, entry cachedToken) bool {
	remaining := entry.expiresAt.Sub(now)
	if entry.lifetime < authShortLifetime {
		return remaining >= entry.lifetime/2
	}
	return remaining >= authRefreshSkew
}

// invalidate drops rejected only when it is still the cached token, so a late 401 keeps a newer token.
// An in-flight refresh is left in place so the retry joins it.
func (a *auth) invalidate(key, rejected string) {
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()

	if entry, ok := a.cache.entries[key]; ok && entry.value == rejected {
		delete(a.cache.entries, key)
	}
}

// token returns a cached access token or the result of a single in-flight fetch for key.
func (a *auth) token(key string, now time.Time, fetch func() (string, time.Duration, error)) (string, error) {
	a.cache.mu.Lock()
	if entry, ok := a.cache.entries[key]; ok && tokenUsable(now, entry) {
		a.cache.mu.Unlock()
		return entry.value, nil
	}
	if flight, ok := a.cache.inflight[key]; ok {
		a.cache.mu.Unlock()
		<-flight.done
		return flight.token, flight.err
	}

	if a.cache.inflight == nil {
		a.cache.inflight = make(map[string]*tokenFetch)
	}
	flight := &tokenFetch{done: make(chan struct{})}
	a.cache.inflight[key] = flight
	a.cache.mu.Unlock()

	token, lifetime, err := fetch()

	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()
	if err == nil && lifetime > 0 {
		if a.cache.entries == nil {
			a.cache.entries = make(map[string]cachedToken)
		}
		// Only this flight writes key, so the entry it replaces is stale even if it expires later.
		a.cache.entries[key] = cachedToken{
			value:     token,
			expiresAt: now.Add(lifetime),
			lifetime:  lifetime,
		}
	}
	a.cache.evictExpired(time.Now())
	delete(a.cache.inflight, key)
	flight.token, flight.lifetime, flight.err = token, lifetime, err
	close(flight.done)
	return token, err
}

// evictExpired bounds the cache when records resolve to many identities. Caller holds mu.
func (c *authCache) evictExpired(now time.Time) {
	for key, entry := range c.entries {
		if !entry.expiresAt.After(now) {
			delete(c.entries, key)
		}
	}
}

// tokenClient stays off the task proxy. API calls still use h.Proxy.
// A redirect that changes scheme, host, or port is not followed: the body carries the client secret.
func (h *httpCore) tokenClient() *http.Client {
	return &http.Client{
		Timeout: time.Duration(h.Timeout),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if len(via) > 0 && !sameEndpoint(req.URL, via[0].URL) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func sameEndpoint(next, prev *url.URL) bool {
	if !strings.EqualFold(next.Scheme, prev.Scheme) || !strings.EqualFold(next.Hostname(), prev.Hostname()) {
		return false
	}
	return endpointPort(next) == endpointPort(prev)
}

func endpointPort(u *url.URL) string {
	if port := u.Port(); port != `` {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case `https`:
		return `443`
	case `http`:
		return `80`
	default:
		return ``
	}
}

func (h *httpCore) fetchToken(resolved *resolvedAuth) (string, time.Duration, error) {
	if resolved.GrantType == grantJWTBearer {
		assertion, err := buildAuthAssertion(resolved)
		if err != nil {
			return ``, 0, err
		}
		resolved.Assertion = assertion
	}

	req, err := resolved.tokenRequest()
	if err != nil {
		return ``, 0, err
	}

	resp, err := h.tokenClient().Do(req)
	if err != nil {
		return ``, 0, fmt.Errorf("auth token request to %s failed: %w", tokenURLForError(resolved.TokenURL), tokenTransportErr(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ``, 0, fmt.Errorf("auth token request to %s failed: %w", tokenURLForError(resolved.TokenURL), err)
	}
	return parseTokenResponse(resp.StatusCode, body, resolved)
}

func parseTokenResponse(status int, body []byte, resolved *resolvedAuth) (string, time.Duration, error) {
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return ``, 0, fmt.Errorf("auth token request to %s failed with status %d", tokenURLForError(resolved.TokenURL), status)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ``, 0, fmt.Errorf("auth token response from %s is not a JSON object", tokenURLForError(resolved.TokenURL))
	}

	token, _ := payload[resolved.TokenField].(string)
	if token == `` {
		return ``, 0, fmt.Errorf("auth token response from %s is missing %s", tokenURLForError(resolved.TokenURL), resolved.TokenField)
	}

	lifetime, _ := parseExpires(payload[resolved.ExpiresField])
	return token, lifetime, nil
}

func (r *resolvedAuth) wireGrant() string {
	if r.GrantType == grantJWTBearer {
		return jwtBearerGrantURN
	}
	return r.GrantType
}

func (r *resolvedAuth) tokenParams() url.Values {
	params := url.Values{}
	params.Set(grantTypeKey, r.wireGrant())

	if r.GrantType == grantRefreshToken {
		name := grantRefreshToken
		if r.ClientAuth == clientAuthQuery {
			name = r.RefreshTokenParam
		}
		params.Set(name, r.RefreshToken)
	}
	if r.Assertion != `` {
		params.Set(assertionKey, r.Assertion)
	}
	if r.Scope != `` {
		params.Set(`scope`, r.Scope)
	}
	if r.ClientAuth != clientAuthBasic && (r.ClientID != `` || r.ClientSecret != ``) {
		idName := defaultClientIDParam
		secretName := defaultClientSecretParam
		if r.ClientAuth == clientAuthQuery {
			idName = r.ClientIDParam
			secretName = r.ClientSecretParam
		}
		if r.ClientID != `` {
			params.Set(idName, r.ClientID)
		}
		if r.ClientSecret != `` {
			params.Set(secretName, r.ClientSecret)
		}
	}
	return params
}

func (r *resolvedAuth) tokenRequest() (*http.Request, error) {
	params := r.tokenParams()
	endpoint := r.TokenURL
	var body io.Reader
	contentType := ``

	switch r.ClientAuth {
	case clientAuthQuery:
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("auth token_url is invalid")
		}
		query := parsed.Query()
		for key, values := range params {
			if len(values) > 0 {
				query.Set(key, values[0])
			}
		}
		parsed.RawQuery = query.Encode()
		endpoint = parsed.String()
		body = http.NoBody
	case clientAuthBody, clientAuthBasic:
		if r.BodyEncoding == bodyEncodingJSON {
			payload := make(map[string]string, len(params))
			for key, values := range params {
				if len(values) > 0 {
					payload[key] = values[0]
				}
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(raw)
			contentType = contentTypeJSON
		} else {
			body = strings.NewReader(params.Encode())
			contentType = contentTypeForm
		}
	default:
		return nil, fmt.Errorf("unsupported auth client_auth: %s", r.ClientAuth)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("auth token_url is invalid")
	}
	if contentType != `` {
		req.Header.Set(contentTypeKey, contentType)
	}
	if r.ClientAuth == clientAuthBasic {
		// RFC 6749 section 2.3.1 form-encodes both values before Base64.
		req.SetBasicAuth(url.QueryEscape(r.ClientID), url.QueryEscape(r.ClientSecret))
	}
	return req, nil
}

func buildAuthAssertion(resolved *resolvedAuth) (string, error) {
	now := time.Now().Unix()
	claims := jwt.MapClaims{
		`iss`: resolved.Issuer,
		`sub`: resolved.Subject,
		`aud`: resolved.Audience,
		`iat`: now,
		`exp`: now + jwtExpiration,
	}
	if resolved.Scope != `` {
		claims[`scope`] = resolved.Scope
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(resolved.PrivateKey))
	if err != nil {
		return ``, fmt.Errorf("auth private_key is invalid")
	}
	signed, err := token.SignedString(key)
	if err != nil {
		return ``, fmt.Errorf("auth private_key is invalid")
	}
	return signed, nil
}

func parseExpires(value any) (time.Duration, bool) {
	switch typed := value.(type) {
	case float64:
		return expiresFromFloat(typed)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return expiresFromFloat(parsed)
	default:
		return 0, false
	}
}

func expiresFromFloat(seconds float64) (time.Duration, bool) {
	if seconds <= 0 {
		return 0, false
	}
	return time.Duration(seconds * float64(time.Second)), true
}

// tokenTransportErr keeps no text from err: a wrapped error can carry a redirect URL with credentials.
func tokenTransportErr(err error) error {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.New("request timed out")
	}
	return errors.New("transport error")
}

func tokenURLForError(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		if index := strings.Index(raw, `?`); index >= 0 {
			return raw[:index]
		}
		return raw
	}
	parsed.User = nil
	parsed.RawQuery = ``
	parsed.Fragment = ``
	return parsed.String()
}
