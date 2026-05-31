package sftp

import (
	"fmt"
	"time"
)

// withRetry runs fn up to attempts times, sleeping delay between tries. It
// returns nil on the first success, or the last error if every attempt fails.
// This gives us simple resilience against the transient connection drops that
// are common with SFTP servers (acceptance criteria: "Add retry and timeout
// handling for unstable connections").
//
// On each failed attempt that will be retried, it logs a warning that includes
// label, so an operator can see when a connection is flaky even though the
// transfer eventually succeeds. The final error is not logged here; the caller
// surfaces it.
//
// We keep a constant delay rather than exponential backoff: SFTP failures are
// usually short network blips, the attempt counts are small, and a predictable
// delay is easier to reason about and test. The HTTP task's backoff
// (internal/pkg/pipeline/task/http/retry.go) is status-code aware and specific
// to HTTP, so it is not reusable here.
func withRetry(label string, attempts int, delay time.Duration, fn func() error) error {

	if attempts < 1 {
		attempts = 1
	}

	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		// Don't log or sleep after the final attempt.
		if i < attempts-1 {
			fmt.Printf("WARN: %s: attempt %d/%d failed: %v; retrying in %s\n", label, i+1, attempts, err, delay)
			time.Sleep(delay)
		}
	}

	return err

}

// retry wraps withRetry with the task's configured attempt count and delay, and
// builds a label from the task name and the action being attempted (for example
// "connect" or "upload /incoming/file.csv").
func (s *sftp) retry(action string, fn func() error) error {
	return withRetry(fmt.Sprintf(`sftp task %q: %s`, s.Name, action), s.MaxRetries, time.Duration(s.RetryDelay), fn)
}
