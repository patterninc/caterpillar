package heimdall

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	// Endpoint paths
	endpointJob       = "/api/v1/job"
	endpointJobStatus = endpointJob + "/%s/status"
	endpointJobResult = endpointJob + "/%s/result"

	// Job status values
	jobStatusSucceeded = "SUCCEEDED"
	jobStatusFailed    = "FAILED"

	// Default values
	defaultPollInterval = duration.Duration(5 * time.Second)
	defaultTimeout      = duration.Duration(5 * time.Minute)
	defaultEndpoint     = "http://localhost:9090"

	// Default job settings
	defaultJobVersion = `0.0.1`
	defaultJobName    = `caterpillar`
)

type heimdall struct {
	task.Base    `yaml:",inline" json:",inline"`
	Endpoint     string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Headers      map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	PollInterval duration.Duration `yaml:"poll_interval,omitempty" json:"poll_interval,omitempty"`
	Timeout      duration.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	JobRequest   *jobRequest       `yaml:"job,omitempty" json:"job,omitempty" validate:"required"`
}

func New() (task.Task, error) {
	h := &heimdall{
		Endpoint:     defaultEndpoint,
		PollInterval: defaultPollInterval,
		Timeout:      defaultTimeout,
		JobRequest: &jobRequest{
			Name:    defaultJobName,
			Version: defaultJobVersion,
		},
	}
	return h, nil
}

func (h *heimdall) Run(ctx context.Context, input <-chan *record.Record, output chan<- *record.Record) (err error) {

	// If input is provided, override the job request context
	if input != nil {
		for {
			rc, ok := h.GetRecord(input)
			if !ok {
				break
			}

			// Parse the input record to get dynamic context
			var jobContext map[string]any
			if err := json.Unmarshal([]byte(rc.Data), &jobContext); err != nil {
				return err
			}

			// Create a job request with the dynamic context
			jobReq := h.buildJobRequest(jobContext)
			if err := h.submitJob(jobReq, output); err != nil {
				return err
			}
		}
		return nil
	}

	// Create a job request with the configured context
	jobReq := h.buildJobRequest(h.JobRequest.Context)
	return h.submitJob(jobReq, output)

}

// buildJobRequest creates a complete job request with all fields including tags
func (h *heimdall) buildJobRequest(context map[string]any) *jobRequest {
	tags := make([]string, 0, len(h.JobRequest.Tags)+1)
	tags = append(tags, h.JobRequest.Tags...)
	tags = append(tags, fmt.Sprintf("caterpillar_task_name:%s", h.Base.Name))

	return &jobRequest{
		Name:            h.JobRequest.Name,
		Version:         h.JobRequest.Version,
		Context:         context,
		CommandCriteria: h.JobRequest.CommandCriteria,
		ClusterCriteria: h.JobRequest.ClusterCriteria,
		Tags:            tags,
	}
}

func (h *heimdall) submitJob(jobReq *jobRequest, output chan<- *record.Record) error {

	response := &response{}

	if err := h.api(http.MethodPost, h.Endpoint+endpointJob, jobReq, response); err != nil {
		return err
	}

	// Fail if heimdall job failed
	if response.Status == jobStatusFailed {
		return fmt.Errorf("job id %s failed", response.ID)
	}

	// If job is synchronous, handle the result immediately
	if response.IsSync {
		return h.sendToOutput(response.Result, output)
	} else {
		// For asynchronous jobs, poll until completion
		return h.processAsyncJob(response.ID, output)
	}

}

func (h *heimdall) processAsyncJob(jobID string, output chan<- *record.Record) error {

	// Set timeout for job polling
	endTime := time.Now().Add(time.Duration(h.Timeout))

	for time.Now().Before(endTime) {
		time.Sleep(time.Duration(h.PollInterval))

		// Poll for job status
		response := &response{}
		if err := h.api(http.MethodGet, fmt.Sprintf(h.Endpoint+endpointJobStatus, jobID), nil, response); err != nil {
			return err
		}

		switch response.Status {
		case jobStatusSucceeded:
			// Get the job result directly from the result endpoint
			result := &result{}
			if err := h.api(http.MethodGet, fmt.Sprintf(h.Endpoint+endpointJobResult, jobID), nil, result); err != nil {
				return err
			}
			return h.sendToOutput(result, output)
		case jobStatusFailed:
			return fmt.Errorf("job id %s failed", jobID)
		}
		// Otherwise job is still running, continue polling
	}

	return fmt.Errorf("job %s timed out after %v", jobID, h.Timeout)

}
