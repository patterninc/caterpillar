package http

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	nonceLength = 24
)

func getNonce() (string, error) {

	b := make([]byte, nonceLength)

	if _, err := rand.Read(b); err != nil {
		return ``, err
	}

	return base64.URLEncoding.EncodeToString(b)[0:nonceLength], nil

}

func sha256Hash(message string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func percentEncode(input string) string {
	var buf bytes.Buffer
	for _, b := range []byte(input) {
		// if in unreserved set
		if shouldEscape(b) {
			buf.Write([]byte(fmt.Sprintf("%%%02X", b)))
		} else {
			// do not escape, write byte as-is
			buf.WriteByte(b)
		}
	}
	return buf.String()
}

func shouldEscape(c byte) bool {
	// RFC3986 2.3 unreserved characters
	if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return false
	}
	switch c {
	case '-', '.', '_', '~':
		return false
	}
	// all other bytes must be escaped
	return true
}

func (h *httpCore) oauth1(endpoint string, r *http.Request) (err error) {

	// let's parse the endpoint we got to...
	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	// ...get the URL query, and...
	urlQuery := parsedURL.Query()

	// ...URL w/o query
	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host)
	parsedURL.RawQuery = ``

	// set oauth parameters
	oauthParameters := map[string]string{
		`oauth_consumer_key`:     h.Oauth.ConsumerKey,
		`oauth_signature_method`: h.Oauth.SignatureMethod,
		`oauth_timestamp`:        fmt.Sprintf("%d", time.Now().Unix()),
		`oauth_token`:            h.Oauth.Token,
		`oauth_version`:          h.Oauth.Version,
	}

	if oauthParameters[`oauth_nonce`], err = getNonce(); err != nil {
		return err
	}

	parameters, authorizationParts := []string{}, []string{}

	for k, v := range oauthParameters {
		parameters = append(parameters, fmt.Sprintf("%s=%s", k, percentEncode(v)))
		authorizationParts = append(authorizationParts, fmt.Sprintf("%s=%q", k, v))
	}

	for k, v := range urlQuery {
		for _, vv := range v {
			parameters = append(parameters, fmt.Sprintf("%s=%s", k, percentEncode(vv)))
		}
	}

	sort.Strings(parameters)

	baseString := strings.Join([]string{h.Method, percentEncode(parsedURL.String()), percentEncode(strings.Join(parameters, `&`))}, "&")
	signature := url.QueryEscape(sha256Hash(baseString, h.Oauth.ConsumerSecret+`&`+h.Oauth.TokenSecret))

	// generate authorization header value
	authorizationParts = append(authorizationParts, fmt.Sprintf("%s=%q", `oauth_signature`, signature))
	if h.Oauth.Realm != `` {
		authorizationParts = append(authorizationParts, fmt.Sprintf("%s=%q", `realm`, h.Oauth.Realm))
	}
	r.Header.Set(headerAuthorization, fmt.Sprintf("OAuth %s", strings.Join(authorizationParts, `,`)))

	return nil

}
