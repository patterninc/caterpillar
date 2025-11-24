package heimdall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (h *heimdall) api(method, url string, jobReq *jobRequest, obj interface{}) (err error) {

	var payloadJson []byte

	// If we have a job request, prepare the payload
	if jobReq != nil && method != http.MethodGet {
		if payloadJson, err = json.Marshal(jobReq); err != nil {
			return err
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(payloadJson))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	// set user supplied headers
	for key, value := range h.Headers {
		req.Header.Set(key, value)
	}

	// Build client
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(responseBody))
	}

	// if caller did not provide object to unmarshal to, bail...
	if obj == nil {
		return nil
	}

	return json.Unmarshal(responseBody, obj)

}
