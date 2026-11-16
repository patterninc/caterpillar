package http

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	grantRefreshToken        = `refresh_token`
	grantClientCredentials   = `client_credentials`
	grantJWTBearer           = `jwt_bearer`
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
	resolved, err := a.readAuth(rc)
	if err != nil {
		return nil, err
	}
	if err := resolved.prepare(); err != nil {
		return nil, err
	}
	return resolved, nil
}

func (a *auth) readAuth(rc *record.Record) (*resolvedAuth, error) {
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
	return resolved, nil
}

func (r *resolvedAuth) prepare() error {
	if r.GrantType == `` {
		return fmt.Errorf("auth grant_type is required")
	}
	if r.TokenURL == `` {
		return fmt.Errorf("auth token_url is required")
	}

	switch r.GrantType {
	case grantRefreshToken:
		if err := requireAuth(r.RefreshToken, `refresh_token`); err != nil {
			return err
		}
		if err := requireClient(r); err != nil {
			return err
		}
	case grantClientCredentials:
		if err := requireClient(r); err != nil {
			return err
		}
	case grantJWTBearer:
		if err := requireAuth(r.PrivateKey, `private_key`); err != nil {
			return err
		}
		if err := requireAuth(r.Issuer, `issuer`); err != nil {
			return err
		}
		if err := requireAuth(r.Subject, `subject`); err != nil {
			return err
		}
		if err := requireAuth(r.Audience, `audience`); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported auth grant_type: %s", r.GrantType)
	}

	r.ClientAuth = nonempty(r.ClientAuth, clientAuthBody)
	switch r.ClientAuth {
	case clientAuthBody, clientAuthBasic, clientAuthQuery:
	default:
		return fmt.Errorf("unsupported auth client_auth: %s", r.ClientAuth)
	}

	if r.ClientAuth == clientAuthBasic {
		if err := requireClient(r); err != nil {
			return err
		}
	}

	if r.ClientAuth != clientAuthQuery {
		r.BodyEncoding = nonempty(r.BodyEncoding, bodyEncodingForm)
		switch r.BodyEncoding {
		case bodyEncodingJSON, bodyEncodingForm:
		default:
			return fmt.Errorf("unsupported auth body_encoding: %s", r.BodyEncoding)
		}
	}

	r.Header = nonempty(r.Header, defaultAuthHeader)
	r.Scheme = nonempty(r.Scheme, authSchemeBearer)
	switch r.Scheme {
	case authSchemeBearer, authSchemeRaw, authSchemeToken:
	default:
		return fmt.Errorf("unsupported auth scheme: %s", r.Scheme)
	}

	r.TokenField = nonempty(r.TokenField, defaultTokenField)
	r.ExpiresField = nonempty(r.ExpiresField, defaultExpiresField)

	if r.ClientAuth == clientAuthQuery {
		r.ClientIDParam = nonempty(r.ClientIDParam, defaultClientIDParam)
		r.ClientSecretParam = nonempty(r.ClientSecretParam, defaultClientSecretParam)
		r.RefreshTokenParam = nonempty(r.RefreshTokenParam, grantRefreshToken)
	}
	return nil
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
	sum := sha256.Sum256([]byte(strings.Join([]string{
		r.GrantType, r.TokenURL, r.ClientID, identity, r.Scope, r.Audience, r.Issuer, r.PrivateKey,
	}, "\n")))
	return hex.EncodeToString(sum[:])
}

func (h *httpCore) applyAuth(req *http.Request, rc *record.Record) error {
	resolved, err := h.Auth.resolve(rc)
	if err != nil {
		return err
	}

	token, err := h.Auth.token(resolved.cacheKey(), time.Now(), func() (string, time.Duration, error) {
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
