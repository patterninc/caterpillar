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

func (h *httpCore) oauth2(endpoint string, r *http.Request, oauth *resolvedOAuth) error {

	jwt, err := getJWT(oauth)
	if err != nil {
		return err
	}

	accessToken, err := getOauthToken(oauth, jwt)
	if err != nil {
		return err
	}

	r.Header.Set(headerAuthorization, fmt.Sprintf(bearerTokenKey, accessToken))

	return nil

}

func getJWT(oauth *resolvedOAuth) (string, error) {

	now := time.Now().Unix()

	claims := jwt.MapClaims{
		"iss":   oauth.Issuer,
		"sub":   oauth.Subject,
		"aud":   oauth.Audience,
		"iat":   now,
		"exp":   now + jwtExpiration,
		"scope": strings.Join(oauth.Scope, " "),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(oauth.PrivateKey))
	if err != nil {
		return ``, err
	}

	return token.SignedString(key)

}

func getOauthToken(oauth *resolvedOAuth, jwt string) (string, error) {

	data := url.Values{}
	data.Set(assertionKey, jwt)
	data.Set(grantTypeKey, oauth.GrantType)
	payload := strings.NewReader(data.Encode())

	req, err := http.NewRequest(http.MethodPost, oauth.TokenURI, payload)
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

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ``, fmt.Errorf("oauth token request failed with status %d", resp.StatusCode)
	}

	var accessTokenResp map[string]interface{}
	if err = json.Unmarshal(body, &accessTokenResp); err != nil {
		return ``, err
	}

	accessToken, ok := accessTokenResp[accessTokenKey].(string)
	if !ok || accessToken == `` {
		return ``, fmt.Errorf("oauth token response missing access_token")
	}

	return accessToken, nil

}
