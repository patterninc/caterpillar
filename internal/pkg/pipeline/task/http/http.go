package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/http/status"
)

const (
	defaultOAuthVersion     = `1.0`
	defaultSignatureMethod  = `HMAC-SHA256`
	defaultExpectedStatuses = `200`
	defaultMethod           = http.MethodGet
	defaultTimeout          = duration.Duration(90 * time.Second)
)

var (
	ctx = context.Background()
)

type oauth struct {
	ConsumerKey     string   `yaml:"consumer_key" json:"consumer_key"`
	ConsumerSecret  string   `yaml:"consumer_secret" json:"consumer_secret"`
	Token           string   `yaml:"token" json:"token"`
	TokenSecret     string   `yaml:"token_secret" json:"token_secret"`
	Version         string   `yaml:"version,omitempty" json:"version,omitempty"`
	SignatureMethod string   `yaml:"signature_method,omitempty" json:"signature_method,omitempty"`
	Realm           string   `yaml:"realm,omitempty" json:"realm,omitempty"`
	PrivateKey      string   `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	Subject         string   `yaml:"subject,omitempty" json:"subject,omitempty"`
	Issuer          string   `yaml:"issuer,omitempty" json:"issuer,omitempty"`
	Audience        string   `yaml:"audience,omitempty" json:"audience,omitempty"`
	TokenURI        string   `yaml:"token_uri,omitempty" json:"token_uri,omitempty"`
	GrantType       string   `yaml:"grant_type,omitempty" json:"grant_type,omitempty"`
	Scope           []string `yaml:"scope,omitempty" json:"scope,omitempty"`
}

type httpCore struct {
	task.Base        `yaml:",inline" json:",inline"`
	Method           string            `yaml:"method,omitempty" json:"method,omitempty"`
	Endpoint         string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Headers          map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	ExpectedStatuses *status.Statuses  `yaml:"expected_statuses,omitempty" json:"expected_statuses,omitempty"`
	Body             string            `yaml:"body,omitempty" json:"body,omitempty"`
	NextPage         *config.String    `yaml:"next_page,omitempty" json:"next_page,omitempty"`
	Oauth            *oauth            `yaml:"oauth,omitempty" json:"oauth,omitempty"`
	Proxy            *proxy            `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	Timeout          duration.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxRetries       int               `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	RetryDelay       duration.Duration `yaml:"retry_delay,omitempty" json:"retry_delay,omitempty"`
}

type result struct {
	Data    string              `json:"data"`
	Headers map[string][]string `json:"headers"`
}

func New() (task.Task, error) {

	expectedStatuses, err := status.New(defaultExpectedStatuses)
	if err != nil {
		return nil, err
	}

	return &httpCore{
		Method:           defaultMethod,
		ExpectedStatuses: expectedStatuses,
		MaxRetries:       defaultMaxRetries,
		RetryDelay:       defaultDelay,
		Timeout:          defaultTimeout,
	}, nil

}

func (h *httpCore) newFromInput(data []byte) (*httpCore, error) {

	newHttp := &httpCore{
		Method:           h.Method,
		Endpoint:         h.Endpoint,
		ExpectedStatuses: h.ExpectedStatuses,
		Body:             h.Body,
		NextPage:         h.NextPage,
		Oauth:            h.Oauth,
		Proxy:            h.Proxy,
		Timeout:          h.Timeout,
		MaxRetries:       h.MaxRetries,
		RetryDelay:       h.RetryDelay,
	}

	if err := json.Unmarshal(data, newHttp); err != nil {
		return nil, fmt.Errorf("cannot parse http payload [%s]: %s", err, string(data))
	}

	// we only append headers that are present in the current task (h) and missing in the new one
	if len(h.Headers) > 0 {
		if newHttp.Headers == nil {
			newHttp.Headers = make(map[string]string)
		}
		for k, v := range h.Headers {
			if _, found := newHttp.Headers[k]; !found {
				newHttp.Headers[k] = v
			}
		}
	}

	return newHttp, nil

}

func (h *httpCore) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	// if we have input, treat each value as a URL and try to get data from it...
	if input != nil {
		for {
			rc, ok := h.GetRecord(input)
			if !ok {
				break
			}

			// let's get our http object
			newHttp, err := h.newFromInput(rc.Data)
			if err != nil {
				return err
			}
			if err := newHttp.processItem(rc, output); err != nil {
				return err
			}
		}
	}

	// now we'll process the task configured item itself...
	return h.processItem(nil, output)

}

func (h *httpCore) processItem(rc *record.Record, output chan<- *record.Record) error {

	// let's set the endpoint from which we start
	endpoint := h.Endpoint

	// if we do not have the endpoint, bail
	if endpoint == `` {
		return nil
	}

	// create a default record context if none provided
	if rc == nil {
		rc = &record.Record{Context: ctx}
	}

	// TODO: perhaps expose the starting page number as a parameter for the task
	pageID := 1

	// we have infinite loop to account for potential pagination
	for {
		result, err := h.call(endpoint)

		if err != nil {
			return err
		}

		if output != nil {
			h.SendData(rc.Context, []byte(result.Data), output)
		}

		// if we do not have a way to define the next page, we bail...
		if h.NextPage == nil {
			break
		}

		// we move to the next page
		pageID++

		nextPage, err := h.NextPage.GetJQ(rc)
		if err != nil {
			return err
		}
		nextPageInput, err := json.Marshal(result)
		if err != nil {
			return err
		}
		nextPageData, err := nextPage.Execute(nextPageInput, map[string]any{
			`page_id`: pageID,
		})

		if err != nil {
			return err
		}

		if nextPageData == nil {
			break
		}

		if nextPageString, ok := nextPageData.(string); ok {
			endpoint = nextPageString
		}

	}

	return nil

}

func (h *httpCore) call(endpoint string) (*result, error) {

	var lastErr error
	for attempt := 1; attempt <= h.MaxRetries; attempt++ {

		// set http request
		request, err := http.NewRequest(h.Method, endpoint, bytes.NewBuffer([]byte(h.Body)))
		if err != nil {
			lastErr = err
			if attempt < h.MaxRetries {
				continue
			}
			break
		}

		// set headers
		for k, v := range h.Headers {
			request.Header.Set(k, v)
		}

		// TODO: support multiple "behaviors" for oauth support
		if h.Oauth != nil {
			// apply default values if version and hash method are missing
			if h.Oauth.Version == `` {
				h.Oauth.Version = defaultOAuthVersion
			}
			if h.Oauth.SignatureMethod == `` {
				h.Oauth.SignatureMethod = defaultSignatureMethod
			}
			if err := h.oauth(endpoint, request); err != nil {
				lastErr = err
				if attempt < h.MaxRetries {
					continue
				}
				break
			}
		}

		// Create HTTP client with proxy configuration if specified
		client := &http.Client{
			Timeout: time.Duration(h.Timeout),
		}

		// Do we use proxy for this one?
		if h.Proxy != nil {
			transport, err := h.Proxy.getTransport()
			if err != nil {
				lastErr = err
				if attempt < h.MaxRetries {
					continue
				}
				break
			}
			client.Transport = transport
		}

		response, err := client.Do(request)
		if err != nil {
			lastErr = err
			if attempt < h.MaxRetries {
				h.handleBackoff(attempt, response)
				continue
			}
			break
		}

		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			lastErr = err
			if attempt < h.MaxRetries {
				continue
			}
			break
		}

		if h.ExpectedStatuses != nil {
			if code := response.StatusCode; !h.ExpectedStatuses.Has(code) {
				lastErr = fmt.Errorf("unexpected http response code [%v %s]: %s", code, http.StatusText(code), string(body))
				if attempt < h.MaxRetries {
					h.handleBackoff(attempt, response)
					continue
				}
				break
			}
		}

		return &result{
			Data:    string(body),
			Headers: response.Header,
		}, nil
	}

	return nil, lastErr
}
