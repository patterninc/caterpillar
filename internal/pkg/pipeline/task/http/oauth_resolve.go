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

func (o *oauth) resolve(r *record.Record) (*resolvedOAuth, error) {
	if o == nil {
		return nil, nil
	}

	version, err := o.Version.Get(r)
	if err != nil {
		return nil, err
	}
	if version == `` {
		version = defaultOAuthVersion
	}

	signatureMethod, err := o.SignatureMethod.Get(r)
	if err != nil {
		return nil, err
	}
	if signatureMethod == `` {
		signatureMethod = defaultSignatureMethod
	}

	consumerKey, err := o.ConsumerKey.Get(r)
	if err != nil {
		return nil, err
	}

	consumerSecret, err := o.ConsumerSecret.Get(r)
	if err != nil {
		return nil, err
	}

	token, err := o.Token.Get(r)
	if err != nil {
		return nil, err
	}

	tokenSecret, err := o.TokenSecret.Get(r)
	if err != nil {
		return nil, err
	}

	realm, err := o.Realm.Get(r)
	if err != nil {
		return nil, err
	}

	privateKey, err := o.PrivateKey.Get(r)
	if err != nil {
		return nil, err
	}

	subject, err := o.Subject.Get(r)
	if err != nil {
		return nil, err
	}

	issuer, err := o.Issuer.Get(r)
	if err != nil {
		return nil, err
	}

	audience, err := o.Audience.Get(r)
	if err != nil {
		return nil, err
	}

	tokenURI, err := o.TokenURI.Get(r)
	if err != nil {
		return nil, err
	}

	grantType, err := o.GrantType.Get(r)
	if err != nil {
		return nil, err
	}

	scope := make([]string, 0, len(o.Scope))
	for _, scopeValue := range o.Scope {
		resolved, err := scopeValue.Get(r)
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
