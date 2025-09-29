package http

import (
	"fmt"
	"net/http"
)

const (
	headerAuthorization = `Authorization`
)

func (h *httpCore) oauth(endpoint string, r *http.Request) error {

	// let's choose the behavior
	behavior, found := map[string]func(string, *http.Request) error{
		`1.0`: h.oauth1,
		`2.0`: h.oauth2,
	}[h.Oauth.Version]

	if !found {
		return fmt.Errorf("unsupported oauth behavior: %v", h.Oauth.Version)
	}

	return behavior(endpoint, r)

}
