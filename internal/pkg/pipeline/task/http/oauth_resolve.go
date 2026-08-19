package http

import (
	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type oauth struct {
	ConsumerKey     config.String   `yaml:"consumer_key" json:"consumer_key"`
	ConsumerSecret  config.String   `yaml:"consumer_secret" json:"consumer_secret"`
	Token           config.String   `yaml:"token" json:"token"`
	TokenSecret     config.String   `yaml:"token_secret" json:"token_secret"`
	Version         config.String   `yaml:"version,omitempty" json:"version,omitempty"`
	SignatureMethod config.String   `yaml:"signature_method,omitempty" json:"signature_method,omitempty"`
	Realm           config.String   `yaml:"realm,omitempty" json:"realm,omitempty"`
	PrivateKey      config.String   `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	Subject         config.String   `yaml:"subject,omitempty" json:"subject,omitempty"`
	Issuer          config.String   `yaml:"issuer,omitempty" json:"issuer,omitempty"`
	Audience        config.String   `yaml:"audience,omitempty" json:"audience,omitempty"`
	TokenURI        config.String   `yaml:"token_uri,omitempty" json:"token_uri,omitempty"`
	GrantType       config.String   `yaml:"grant_type,omitempty" json:"grant_type,omitempty"`
	Scope           []config.String `yaml:"scope,omitempty" json:"scope,omitempty"`
}

type resolvedOAuth struct {
	ConsumerKey     string
	ConsumerSecret  string
	Token           string
	TokenSecret     string
	Version         string
	SignatureMethod string
	Realm           string
	PrivateKey      string
	Subject         string
	Issuer          string
	Audience        string
	TokenURI        string
	GrantType       string
	Scope           []string
}

func (o *oauth) copy() *oauth {
	if o == nil {
		return nil
	}

	cp := *o
	if len(o.Scope) > 0 {
		cp.Scope = append([]config.String(nil), o.Scope...)
	}

	return &cp
}

func resolveString(field string, value config.String, r *record.Record) (string, error) {
	resolved, err := value.Get(r)
	if err != nil {
		return ``, err
	}

	if config.HasUnresolvedContextPlaceholders(resolved) {
		return ``, config.ErrUnresolvedContextPlaceholder(field)
	}

	return resolved, nil
}

func (o *oauth) resolve(r *record.Record) (*resolvedOAuth, error) {
	if o == nil {
		return nil, nil
	}

	version, err := resolveString("oauth.version", o.Version, r)
	if err != nil {
		return nil, err
	}
	if version == `` {
		version = defaultOAuthVersion
	}

	signatureMethod, err := resolveString("oauth.signature_method", o.SignatureMethod, r)
	if err != nil {
		return nil, err
	}
	if signatureMethod == `` {
		signatureMethod = defaultSignatureMethod
	}

	consumerKey, err := resolveString("oauth.consumer_key", o.ConsumerKey, r)
	if err != nil {
		return nil, err
	}

	consumerSecret, err := resolveString("oauth.consumer_secret", o.ConsumerSecret, r)
	if err != nil {
		return nil, err
	}

	token, err := resolveString("oauth.token", o.Token, r)
	if err != nil {
		return nil, err
	}

	tokenSecret, err := resolveString("oauth.token_secret", o.TokenSecret, r)
	if err != nil {
		return nil, err
	}

	realm, err := resolveString("oauth.realm", o.Realm, r)
	if err != nil {
		return nil, err
	}

	privateKey, err := resolveString("oauth.private_key", o.PrivateKey, r)
	if err != nil {
		return nil, err
	}

	subject, err := resolveString("oauth.subject", o.Subject, r)
	if err != nil {
		return nil, err
	}

	issuer, err := resolveString("oauth.issuer", o.Issuer, r)
	if err != nil {
		return nil, err
	}

	audience, err := resolveString("oauth.audience", o.Audience, r)
	if err != nil {
		return nil, err
	}

	tokenURI, err := resolveString("oauth.token_uri", o.TokenURI, r)
	if err != nil {
		return nil, err
	}

	grantType, err := resolveString("oauth.grant_type", o.GrantType, r)
	if err != nil {
		return nil, err
	}

	scope := make([]string, 0, len(o.Scope))
	for _, scopeValue := range o.Scope {
		resolved, err := resolveString("oauth.scope", scopeValue, r)
		if err != nil {
			return nil, err
		}
		if resolved != `` {
			scope = append(scope, resolved)
		}
	}

	return &resolvedOAuth{
		ConsumerKey:     consumerKey,
		ConsumerSecret:  consumerSecret,
		Token:           token,
		TokenSecret:     tokenSecret,
		Version:         version,
		SignatureMethod: signatureMethod,
		Realm:           realm,
		PrivateKey:      privateKey,
		Subject:         subject,
		Issuer:          issuer,
		Audience:        audience,
		TokenURI:        tokenURI,
		GrantType:       grantType,
		Scope:           scope,
	}, nil
}
