package heimdall

type response struct {
	ID     string  `json:"id,omitempty"`
	Status string  `json:"status,omitempty"`
	IsSync bool    `json:"is_sync,omitempty"`
	Result *result `json:"result,omitempty"`
}
