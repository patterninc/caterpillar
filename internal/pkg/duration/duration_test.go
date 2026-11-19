package duration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalYAML(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		expected Duration
		wantErr  bool
	}{
		{
			name:     "Seconds",
			input:    "d: 15s",
			expected: Duration(15 * time.Second),
		},
		{
			name:     "Compound",
			input:    "d: 1h30m",
			expected: Duration(90 * time.Minute),
		},
		{
			name:     "Milliseconds",
			input:    "d: 250ms",
			expected: Duration(250 * time.Millisecond),
		},
		{
			name:     "Zero",
			input:    "d: 0s",
			expected: 0,
		},
		{
			name:    "Bare number has no unit",
			input:   "d: 15",
			wantErr: true,
		},
		{
			name:    "Garbage",
			input:   "d: soon",
			wantErr: true,
		},
		{
			name:    "Not a scalar",
			input:   "d: [1s]",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var holder struct {
				D Duration `yaml:"d"`
			}
			err := yaml.Unmarshal([]byte(tt.input), &holder)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, holder.D)
		})
	}

}

func TestUnmarshalYAMLKeepsDefaultWhenAbsent(t *testing.T) {

	holder := struct {
		D Duration `yaml:"d"`
	}{D: Duration(time.Second)}

	require.NoError(t, yaml.Unmarshal([]byte("other: 1"), &holder))
	assert.Equal(t, Duration(time.Second), holder.D)

}
