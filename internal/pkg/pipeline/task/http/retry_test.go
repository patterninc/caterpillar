package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/http/status"
)

// throttlingServer answers 429 to the first `throttles` requests (with Retry-After when set), then 200.
func throttlingServer(t *testing.T, throttles int32, retryAfter string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= throttles {
			if retryAfter != "" {
				w.Header().Set(retryAfterHeader, retryAfter)
			}
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func newCore(t *testing.T, maxRetries, maxThrottleRetries int) *httpCore {
	t.Helper()
	expected, err := status.New("200")
	if err != nil {
		t.Fatal(err)
	}
	return &httpCore{
		Method:             http.MethodGet,
		ExpectedStatuses:   expected,
		MaxRetries:         maxRetries,
		MaxThrottleRetries: maxThrottleRetries,
		Timeout:            defaultTimeout,
	}
}

func TestCall(t *testing.T) {
	tests := []struct {
		name               string
		throttles          int32
		retryAfter         string
		maxRetries         int
		maxThrottleRetries int
		wantErr            bool
		wantCalls          int32
	}{
		{"throttles beyond max_retries succeed within the throttle budget", 4, "0", 2, 5, false, 5},
		{"without a throttle budget throttles use max_retries", 4, "0", 2, 0, true, 2},
		{"throttle budget then max_retries are both exhausted", 100, "0", 2, 3, true, 5},
		{"429 without Retry-After uses max_retries", 100, "", 1, 5, true, 1},
		{"unparseable Retry-After uses max_retries", 100, "soon", 1, 5, true, 1},
		{"Retry-After above the cap uses max_retries", 100, "61", 1, 5, true, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, calls := throttlingServer(t, tc.throttles, tc.retryAfter)
			res, err := newCore(t, tc.maxRetries, tc.maxThrottleRetries).call(server.URL, &record.Record{})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && res.Data != "ok" {
				t.Fatalf("data = %q, want ok", res.Data)
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("calls = %d, want %d", got, tc.wantCalls)
			}
		})
	}
}

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		status int
		header string
		ok     bool
	}{
		{http.StatusTooManyRequests, "2", true},
		{http.StatusTooManyRequests, "", false},
		{http.StatusTooManyRequests, "-1", false},
		{http.StatusServiceUnavailable, "2", false},
	}
	for _, tc := range tests {
		response := &http.Response{StatusCode: tc.status, Header: http.Header{}}
		if tc.header != "" {
			response.Header.Set(retryAfterHeader, tc.header)
		}
		if _, ok := retryAfter(response); ok != tc.ok {
			t.Errorf("retryAfter(%d, %q) ok = %v, want %v", tc.status, tc.header, ok, tc.ok)
		}
	}
	if _, ok := retryAfter(nil); ok {
		t.Error("retryAfter(nil) ok = true")
	}
}
