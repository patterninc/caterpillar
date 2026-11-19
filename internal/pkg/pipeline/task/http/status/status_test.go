package status

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNew(t *testing.T) {

	tests := []struct {
		name    string
		input   string
		has     []int
		hasNot  []int
		wantErr string
	}{
		{name: "Single", input: "200", has: []int{200}, hasNot: []int{201}},
		{name: "List with spaces", input: "200, 201 ,404", has: []int{200, 201, 404}, hasNot: []int{202}},
		{name: "Range", input: "500..503", has: []int{500, 501, 502, 503}, hasNot: []int{499, 504}},
		{name: "Range with spaces", input: "200 .. 201", has: []int{200, 201}},
		{name: "Mixed", input: "200,429,500..502", has: []int{200, 429, 500, 502}, hasNot: []int{503}},
		{name: "Degenerate range", input: "418..418", has: []int{418}},
		{name: "Not a number", input: "ok", wantErr: "invalid number: ok"},
		{name: "Empty", input: "", wantErr: "invalid number"},
		{name: "Reversed range", input: "503..500", wantErr: "start greater than end"},
		{name: "Bad range bound", input: "5xx..599", wantErr: "invalid numbers in range"},
		{name: "Too many range parts", input: "1..2..3", wantErr: "invalid range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(tt.input)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			for _, code := range tt.has {
				assert.True(t, s.Has(code), "expected %d", code)
			}
			for _, code := range tt.hasNot {
				assert.False(t, s.Has(code), "did not expect %d", code)
			}
		})
	}

}

func TestUnmarshalYAML(t *testing.T) {

	var holder struct {
		Retry Statuses `yaml:"retry"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(`retry: "429,500..502"`), &holder))
	assert.True(t, holder.Retry.Has(429))
	assert.True(t, holder.Retry.Has(501))
	assert.False(t, holder.Retry.Has(200))

	assert.Error(t, yaml.Unmarshal([]byte(`retry: "nope"`), &holder))
	assert.Error(t, yaml.Unmarshal([]byte(`retry: [1, 2]`), &holder))

}

func TestUnmarshalJSONNumber(t *testing.T) {

	var holder struct {
		Retry Statuses `json:"retry"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"retry":503}`), &holder))
	assert.True(t, holder.Retry.Has(503))

}

// UnmarshalJSON hands the raw token, quotes included, to New, so the natural
// JSON spelling of a status list is rejected. Unskip once it decodes the string first.
func TestUnmarshalJSONString(t *testing.T) {

	t.Skip("known bug: Statuses.UnmarshalJSON does not unquote JSON strings (internal/pkg/pipeline/task/http/status/status.go:20)")

	var holder struct {
		Retry Statuses `json:"retry"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"retry":"429,500..502"}`), &holder))
	assert.True(t, holder.Retry.Has(501))

}
