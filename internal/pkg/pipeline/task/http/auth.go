package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	grantRefreshToken        = `refresh_token`
	grantClientCredentials   = `client_credentials`
	grantJWTBearer           = `jwt_bearer`
	jwtBearerGrantURN        = `urn:ietf:params:oauth:grant-type:jwt-bearer`
	clientAuthBody           = `body`
	clientAuthBasic          = `basic`
	clientAuthQuery          = `query`
	bodyEncodingJSON         = `json`
	bodyEncodingForm         = `form`
	authSchemeBearer         = `bearer`
	authSchemeRaw            = `raw`
	authSchemeToken          = `token`
	defaultAuthHeader        = `Authorization`
	defaultTokenField        = `access_token`
	defaultExpiresField      = `expires_in`
	defaultClientIDParam     = `client_id`
	defaultClientSecretParam = `client_secret`
	contentTypeJSON          = `application/json`
	authRefreshSkew          = 30 * time.Second
	authShortLifetime        = 60 * time.Second
)

// errAuthAndOAuth is a task configuration error. ignore_error does not apply.
var errAuthAndOAuth = fmt.Errorf("http task cannot set both oauth and auth")

// auth fetches a short-lived access token for the API request.
// The cache is shared by every worker on this task. Per-record cores must keep
// this pointer: replacing it drops the cache and the task configuration.
type auth struct {
	GrantType         config.String `yaml:"grant_type,omitempty" json:"grant_type,omitempty"`
	TokenURL          config.String `yaml:"token_url,omitempty" json:"token_url,omitempty"`
	ClientAuth        config.String `yaml:"client_auth,omitempty" json:"client_auth,omitempty"`
	BodyEncoding      config.String `yaml:"body_encoding,omitempty" json:"body_encoding,omitempty"`
	RefreshToken      config.String `yaml:"refresh_token,omitempty" json:"refresh_token,omitempty"`
	ClientID          config.String `yaml:"client_id,omitempty" json:"client_id,omitempty"`
	ClientSecret      config.String `yaml:"client_secret,omitempty" json:"client_secret,omitempty"`
	Scope             config.String `yaml:"scope,omitempty" json:"scope,omitempty"`
	ClientIDParam     config.String `yaml:"client_id_param,omitempty" json:"client_id_param,omitempty"`
	ClientSecretParam config.String `yaml:"client_secret_param,omitempty" json:"client_secret_param,omitempty"`
	RefreshTokenParam config.String `yaml:"refresh_token_param,omitempty" json:"refresh_token_param,omitempty"`
	Header            config.String `yaml:"header,omitempty" json:"header,omitempty"`
	Scheme            config.String `yaml:"scheme,omitempty" json:"scheme,omitempty"`
	TokenField        config.String `yaml:"token_field,omitempty" json:"token_field,omitempty"`
	ExpiresField      config.String `yaml:"expires_field,omitempty" json:"expires_field,omitempty"`
	PrivateKey        config.String `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	Issuer            config.String `yaml:"issuer,omitempty" json:"issuer,omitempty"`
	Subject           config.String `yaml:"subject,omitempty" json:"subject,omitempty"`
	Audience          config.String `yaml:"audience,omitempty" json:"audience,omitempty"`
	cache             authCache
}

type authCache struct {
	mu       sync.Mutex
	entries  map[string]cachedToken
	gen      map[string]uint64
	inflight map[string]*tokenFetch
}

// tokenFetch is one in-flight token HTTP call. Waiters block on done.
// gen is the cache generation captured when the fetch started.
type tokenFetch struct {
	gen      uint64
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

type resolvedAuth struct {
	GrantType         string
	TokenURL          string
	ClientAuth        string
	BodyEncoding      string
	RefreshToken      string
	ClientID          string
	ClientSecret      string
	Scope             string
	ClientIDParam     string
	ClientSecretParam string
	RefreshTokenParam string
	Header            string
	Scheme            string
	TokenField        string
	ExpiresField      string
	PrivateKey        string
	Issuer            string
	Subject           string
	Audience          string
	Assertion         string
}

func (h *httpCore) rejectCombinedAuth() error {
	if h.Oauth != nil && h.Auth != nil {
		return errAuthAndOAuth
	}
	return nil
}

func (a *auth) resolve(rc *record.Record) (*resolvedAuth, error) {
	if a == nil {
		return nil, nil
	}

	resolved := &resolvedAuth{}
	fields := []struct {
		name string
		src  config.String
		dst  *string
	}{
		{`grant_type`, a.GrantType, &resolved.GrantType},
		{`token_url`, a.TokenURL, &resolved.TokenURL},
		{`client_auth`, a.ClientAuth, &resolved.ClientAuth},
		{`body_encoding`, a.BodyEncoding, &resolved.BodyEncoding},
		{`refresh_token`, a.RefreshToken, &resolved.RefreshToken},
		{`client_id`, a.ClientID, &resolved.ClientID},
		{`client_secret`, a.ClientSecret, &resolved.ClientSecret},
		{`scope`, a.Scope, &resolved.Scope},
		{`client_id_param`, a.ClientIDParam, &resolved.ClientIDParam},
		{`client_secret_param`, a.ClientSecretParam, &resolved.ClientSecretParam},
		{`refresh_token_param`, a.RefreshTokenParam, &resolved.RefreshTokenParam},
		{`header`, a.Header, &resolved.Header},
		{`scheme`, a.Scheme, &resolved.Scheme},
		{`token_field`, a.TokenField, &resolved.TokenField},
		{`expires_field`, a.ExpiresField, &resolved.ExpiresField},
		{`private_key`, a.PrivateKey, &resolved.PrivateKey},
		{`issuer`, a.Issuer, &resolved.Issuer},
		{`subject`, a.Subject, &resolved.Subject},
		{`audience`, a.Audience, &resolved.Audience},
	}

	for _, field := range fields {
		value, err := field.src.Get(rc)
		if err != nil {
			return nil, fmt.Errorf("auth %s: %w", field.name, err)
		}
		*field.dst = value
	}

	if resolved.GrantType == `` {
		return nil, fmt.Errorf("auth grant_type is required")
	}
	if resolved.TokenURL == `` {
		return nil, fmt.Errorf("auth token_url is required")
	}

	switch resolved.GrantType {
	case grantRefreshToken:
		if err := requireAuth(resolved.RefreshToken, `refresh_token`); err != nil {
			return nil, err
		}
		if err := requireClient(resolved); err != nil {
			return nil, err
		}
	case grantClientCredentials:
		if err := requireClient(resolved); err != nil {
			return nil, err
		}
	case grantJWTBearer:
		if err := requireAuth(resolved.PrivateKey, `private_key`); err != nil {
			return nil, err
		}
		if err := requireAuth(resolved.Issuer, `issuer`); err != nil {
			return nil, err
		}
		if err := requireAuth(resolved.Subject, `subject`); err != nil {
			return nil, err
		}
		if err := requireAuth(resolved.Audience, `audience`); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported auth grant_type: %s", resolved.GrantType)
	}

	resolved.ClientAuth = nonempty(resolved.ClientAuth, clientAuthBody)
	switch resolved.ClientAuth {
	case clientAuthBody, clientAuthBasic, clientAuthQuery:
	default:
		return nil, fmt.Errorf("unsupported auth client_auth: %s", resolved.ClientAuth)
	}

	if resolved.ClientAuth == clientAuthBasic {
		if err := requireClient(resolved); err != nil {
			return nil, err
		}
	}

	if resolved.ClientAuth != clientAuthQuery {
		resolved.BodyEncoding = nonempty(resolved.BodyEncoding, bodyEncodingForm)
		switch resolved.BodyEncoding {
		case bodyEncodingJSON, bodyEncodingForm:
		default:
			return nil, fmt.Errorf("unsupported auth body_encoding: %s", resolved.BodyEncoding)
		}
	}

	resolved.Header = nonempty(resolved.Header, defaultAuthHeader)
	resolved.Scheme = nonempty(resolved.Scheme, authSchemeBearer)
	switch resolved.Scheme {
	case authSchemeBearer, authSchemeRaw, authSchemeToken:
	default:
		return nil, fmt.Errorf("unsupported auth scheme: %s", resolved.Scheme)
	}

	resolved.TokenField = nonempty(resolved.TokenField, defaultTokenField)
	resolved.ExpiresField = nonempty(resolved.ExpiresField, defaultExpiresField)

	if resolved.ClientAuth == clientAuthQuery {
		resolved.ClientIDParam = nonempty(resolved.ClientIDParam, defaultClientIDParam)
		resolved.ClientSecretParam = nonempty(resolved.ClientSecretParam, defaultClientSecretParam)
		resolved.RefreshTokenParam = nonempty(resolved.RefreshTokenParam, grantRefreshToken)
	}

	return resolved, nil
}

func requireAuth(value, name string) error {
	if value == `` {
		return fmt.Errorf("auth %s is required", name)
	}
	return nil
}

func requireClient(resolved *resolvedAuth) error {
	if err := requireAuth(resolved.ClientID, `client_id`); err != nil {
		return err
	}
	return requireAuth(resolved.ClientSecret, `client_secret`)
}

func nonempty(value, fallback string) string {
	if value == `` {
		return fallback
	}
	return value
}

// cacheKey is a digest so the refresh token and subject are not stored as a map key.
// Do not log the inputs or the digest.
func (r *resolvedAuth) cacheKey() string {
	identity := r.RefreshToken
	if r.GrantType == grantJWTBearer {
		identity = r.Subject
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{r.GrantType, r.TokenURL, r.ClientID, identity}, "\n")))
	return hex.EncodeToString(sum[:])
}

func tokenUsable(now time.Time, entry cachedToken) bool {
	remaining := entry.expiresAt.Sub(now)
	if entry.lifetime < authShortLifetime {
		return remaining >= entry.lifetime/2
	}
	return remaining >= authRefreshSkew
}

func (a *auth) invalidate(key string) {
	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()

	if a.cache.gen == nil {
		a.cache.gen = make(map[string]uint64)
	}
	a.cache.gen[key]++
	delete(a.cache.entries, key)
	// The fetch already running keeps its pointer. Its store is dropped because gen moved.
	delete(a.cache.inflight, key)
}

// token returns a cached access token or the result of a single in-flight fetch for key.
// force skips the cache and any fetch already running.
func (a *auth) token(key string, now time.Time, force bool, fetch func() (string, time.Duration, error)) (string, error) {
	a.cache.mu.Lock()
	if !force {
		if entry, ok := a.cache.entries[key]; ok && tokenUsable(now, entry) {
			a.cache.mu.Unlock()
			return entry.value, nil
		}
		if flight, ok := a.cache.inflight[key]; ok {
			a.cache.mu.Unlock()
			<-flight.done
			return flight.token, flight.err
		}
	}

	if a.cache.gen == nil {
		a.cache.gen = make(map[string]uint64)
	}
	if a.cache.inflight == nil {
		a.cache.inflight = make(map[string]*tokenFetch)
	}
	flight := &tokenFetch{gen: a.cache.gen[key], done: make(chan struct{})}
	a.cache.inflight[key] = flight
	a.cache.mu.Unlock()

	token, lifetime, err := fetch()

	a.cache.mu.Lock()
	defer a.cache.mu.Unlock()
	if err == nil && lifetime > 0 && a.cache.gen[key] == flight.gen {
		if a.cache.entries == nil {
			a.cache.entries = make(map[string]cachedToken)
		}
		// A fetch that started earlier must not replace a token that expires later.
		expiresAt := now.Add(lifetime)
		if current, ok := a.cache.entries[key]; !ok || !current.expiresAt.After(expiresAt) {
			a.cache.entries[key] = cachedToken{
				value:     token,
				expiresAt: expiresAt,
				lifetime:  lifetime,
			}
		}
	}
	if a.cache.inflight[key] == flight {
		delete(a.cache.inflight, key)
	}
	flight.token, flight.lifetime, flight.err = token, lifetime, err
	close(flight.done)
	return token, err
}

func (h *httpCore) applyAuth(req *http.Request, rc *record.Record, force bool) error {
	resolved, err := h.Auth.resolve(rc)
	if err != nil {
		return err
	}

	token, err := h.Auth.token(resolved.cacheKey(), time.Now(), force, func() (string, time.Duration, error) {
		return h.fetchToken(resolved)
	})
	if err != nil {
		return err
	}
	return setAuthHeader(req, resolved, token)
}

func (h *httpCore) invalidateAuth(rc *record.Record) error {
	resolved, err := h.Auth.resolve(rc)
	if err != nil {
		return err
	}
	h.Auth.invalidate(resolved.cacheKey())
	return nil
}

func setAuthHeader(req *http.Request, resolved *resolvedAuth, token string) error {
	value, err := formatAuthToken(resolved.Scheme, token)
	if err != nil {
		return err
	}
	req.Header.Set(resolved.Header, value)
	return nil
}

func formatAuthToken(scheme, token string) (string, error) {
	switch scheme {
	case ``, authSchemeBearer:
		return fmt.Sprintf(bearerTokenKey, token), nil
	case authSchemeRaw:
		return token, nil
	case authSchemeToken:
		return `Token ` + token, nil
	default:
		return ``, fmt.Errorf("unsupported auth scheme: %s", scheme)
	}
}

// tokenClient stays off the task proxy. API calls still use h.Proxy.
func (h *httpCore) tokenClient() *http.Client {
	return &http.Client{Timeout: time.Duration(h.Timeout)}
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

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ``, 0, fmt.Errorf("auth token request to %s failed with status %d", tokenURLForError(resolved.TokenURL), resp.StatusCode)
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

func (r *resolvedAuth) tokenRequest() (*http.Request, error) {
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
		req.SetBasicAuth(r.ClientID, r.ClientSecret)
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

// tokenTransportErr drops the request URL. *url.Error includes the query string and user info.
func tokenTransportErr(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}
	return err
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
