package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwtExpiration   = 30 * 60 // seconds
	bearerTokenKey  = `Bearer %s`
	assertionKey    = `assertion`
	grantTypeKey    = `grant_type`
	accessTokenKey  = `access_token`
	contentTypeKey  = `Content-Type`
	contentTypeForm = `application/x-www-form-urlencoded`
)

func (h *httpCore) oauth2(endpoint string, r *http.Request) error {

	jwt, err := h.getJWT()
	if err != nil {
		return err
	}

	accessToken, err := h.getOauthToken(jwt)
	if err != nil {
		return err
	}

	r.Header.Set(headerAuthorization, fmt.Sprintf(bearerTokenKey, accessToken))

	return nil

}

func (h *httpCore) getJWT() (string, error) {

	now := time.Now().Unix()

	claims := jwt.MapClaims{
		"iss":   h.Oauth.Issuer,
		"sub":   h.Oauth.Subject,
		"aud":   h.Oauth.Audience,
		"iat":   now,
		"exp":   now + jwtExpiration,
		"scope": strings.Join(h.Oauth.Scope, " "),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// parse private key
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(h.Oauth.PrivateKey))
	if err != nil {
		return ``, err
	}

	return token.SignedString(key)

}

func (h *httpCore) getOauthToken(jwt string) (string, error) {

	data := url.Values{}
	data.Set(assertionKey, jwt)
	data.Set(grantTypeKey, h.Oauth.GrantType)
	payload := strings.NewReader(data.Encode())

	req, err := http.NewRequest(http.MethodPost, h.Oauth.TokenURI, payload)
	if err != nil {
		return ``, err
	}

	req.Header.Set(contentTypeKey, contentTypeForm)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ``, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ``, err
	}

	var accessTokenResp map[string]interface{}
	if err = json.Unmarshal(body, &accessTokenResp); err != nil {
		return ``, err
	}

	return accessTokenResp[accessTokenKey].(string), nil

}
