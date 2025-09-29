package http

import (
	"math"
	"net/http"
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

func (h *httpCore) handleBackoff(attempt int, response *http.Response) {

	statusCode := getStatusCode(response)

	behaviors := map[int]func(int, *http.Response){
		http.StatusTooManyRequests: func(attempt int, response *http.Response) {

			if response != nil {
				if response.Header.Get(retryAfterHeader) != "" {
					retryAfter := response.Header.Get(retryAfterHeader)
					retryAfterDuration, err := time.ParseDuration(retryAfter + "s")
					if err == nil {
						time.Sleep(retryAfterDuration)
					}
					return
				}
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
