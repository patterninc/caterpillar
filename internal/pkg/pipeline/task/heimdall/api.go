package heimdall

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var (
	headers = http.Header{
		`Content-Type`: {`application/json`},
	}
)

func (h *heimdall) api(method, url string, obj interface{}) (err error) {

	var payloadJson []byte

	// let's set additional job tags
	h.JobRequest.Tags = append(h.JobRequest.Tags, fmt.Sprintf("caterpillar_task_name:%s", h.Base.Name))

	if method != http.MethodGet {
		if payloadJson, err = json.Marshal(&h.JobRequest); err != nil {
			return err
		}
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(payloadJson))
	if err != nil {
		return err
	}

	// Set default headers
	req.Header = headers

	// set user supplied headers
	for key, value := range h.Headers {
		req.Header.Add(key, value)
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
