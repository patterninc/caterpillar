package http

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
)

const (
	retryAfterHeader  = "Retry-After"
	defaultMaxRetries = 3
	defaultDelay      = duration.Duration(1 * time.Second)
	statusNilResponse = -1
)

func getStatusCode(response *http.Response) int {
	if response == nil {
		return statusNilResponse
	}
	return response.StatusCode
}

// retryAfter returns the delay a 429 response asks for in its Retry-After header (delay-seconds only).
func retryAfter(response *http.Response) (time.Duration, bool) {
	if response == nil || response.StatusCode != http.StatusTooManyRequests {
		return 0, false
	}
	seconds, err := strconv.ParseUint(strings.TrimSpace(response.Header.Get(retryAfterHeader)), 10, 32)
	if err != nil {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

func (h *httpCore) handleBackoff(attempt int, response *http.Response) {

	statusCode := getStatusCode(response)

	behaviors := map[int]func(int, *http.Response){
		http.StatusTooManyRequests: func(attempt int, response *http.Response) {

			if delay, ok := retryAfter(response); ok {
				time.Sleep(delay)
				return
			}
			backoff := math.Pow(2, float64(attempt))
			time.Sleep(time.Duration(backoff) * time.Second)
		},
		statusNilResponse: func(attempt int, response *http.Response) {
			time.Sleep(time.Duration(h.RetryDelay))
		},
		// Add more status codes and their backoff logic here if needed
	}

	behavior, found := behaviors[statusCode]
	if !found {
		time.Sleep(time.Duration(h.RetryDelay))
		return
	}
	behavior(attempt, response)
}
