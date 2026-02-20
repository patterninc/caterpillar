package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	contentTypeKey          = `Content-Type`
	contentTypeJson         = `application/json`
	errorReadingRequestBody = `error reading request body`
	errorInternalServer     = `internal server error`
)

var (
	ctx       = context.Background()
	okMessage = []byte(`{"ok":true}`)
)

type requestPayload struct {
	Method  string              `yaml:"method,omitempty" json:"method,omitempty"`
	Path    string              `yaml:"path,omitempty" json:"path,omitempty"`
	Query   map[string][]string `yaml:"query,omitempty" json:"query,omitempty"`
	Body    string              `yaml:"body,omitempty" json:"body,omitempty"`
	Headers map[string][]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// createPathHandler creates an individual handler for a specific path configuration
func (s *server) createPathHandler(pathConfig pathConfig, output chan<- *record.Record) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		// Check if the HTTP method matches
		if r.Method != pathConfig.Method {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			// TODO: log the actual error on the server side
			http.Error(w, errorReadingRequestBody, http.StatusBadRequest)
			return
		}

		// create request data
		data := &requestPayload{
			Method:  r.Method,
			Path:    pathConfig.Path,
			Query:   r.URL.Query(),
			Body:    string(body),
			Headers: r.Header,
		}

		// convert to json
		jsonData, err := json.Marshal(data)
		if err != nil {
			// TODO: log the actual error on the server side
			http.Error(w, errorInternalServer, http.StatusInternalServerError)
			return
		}

		// create a new record with the request information
		s.SendData(nil, jsonData, output)

		w.Header().Set(contentTypeKey, contentTypeJson)
		w.WriteHeader(http.StatusOK)
		w.Write(okMessage)
	}
}
