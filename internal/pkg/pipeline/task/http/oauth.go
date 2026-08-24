package http

import (
	"fmt"
	"net/http"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	headerAuthorization = `Authorization`
)

func (h *httpCore) oauth(endpoint string, r *http.Request, rc *record.Record) error {

	resolved, err := h.Oauth.resolve(rc)
	if err != nil {
		return err
	}

	behavior, found := map[string]func(string, *http.Request, *resolvedOAuth) error{
		`1.0`: h.oauth1,
		`2.0`: h.oauth2,
	}[resolved.Version]

	if !found {
		return fmt.Errorf("unsupported oauth behavior: %v", resolved.Version)
	}

	return behavior(endpoint, r, resolved)

}
