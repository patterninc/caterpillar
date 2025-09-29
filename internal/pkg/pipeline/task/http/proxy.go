package http

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	httpScheme  = "http"
	httpsScheme = "https"
)

var (
	errCertPath    = fmt.Errorf("CA certificate path is required when using TLS")
	errCertParsing = fmt.Errorf("failed to parse CA certificate")
)

type proxy struct {
	Scheme        string `yaml:"scheme" json:"scheme"`
	Host          string `yaml:"host" json:"host"`
	Username      string `yaml:"username,omitempty" json:"username,omitempty"`
	Password      string `yaml:"password,omitempty" json:"password,omitempty"`
	CACertificate string `yaml:"ca_certificate,omitempty" json:"ca_certificate,omitempty"`
	InsecreTLS    bool   `yaml:"insecure_tls,omitempty" json:"insecure_tls,omitempty"`
}

// createTLSFromCert creates a TLS configuration based on the proxy settings
func (p *proxy) getTLSConfig() (*tls.Config, error) {

	p.CACertificate = strings.ReplaceAll(p.CACertificate, "\\n", "\n")

	if len(p.CACertificate) == 0 {
		return nil, errCertPath
	}

	caCertPool := x509.NewCertPool()

	if !caCertPool.AppendCertsFromPEM([]byte(p.CACertificate)) {
		return nil, errCertParsing
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: p.InsecreTLS,
		RootCAs:            caCertPool,
	}

	return tlsConfig, nil

}

func (p *proxy) getTransport() (*http.Transport, error) {

	// let's set starting transport
	t := &http.Transport{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: p.Scheme,
			Host:   p.Host,
			User:   url.UserPassword(p.Username, p.Password),
		}),
	}

	switch p.Scheme {
	case httpScheme:
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: p.InsecreTLS}
	case httpsScheme:
		var err error
		if t.TLSClientConfig, err = p.getTLSConfig(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", p.Scheme)
	}

	return t, nil

}
