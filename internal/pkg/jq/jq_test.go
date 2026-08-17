package jq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, path string) *Query {
	t.Helper()

	q := &Query{}
	require.NoError(t, q.parse(func(obj any) error {
		*(obj.(*string)) = path
		return nil
	}))

	return q
}

func TestExecuteError(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		document    string
		expectedErr string
		expected    any
	}{
		{
			name:        "error() is propagated",
			path:        `error("something went wrong")`,
			document:    `"Aadhya"`,
			expectedErr: "something went wrong",
		},
		{
			name:        "conditional error() is propagated",
			path:        `if . == "Aadhya" then error("something went wrong") else . end`,
			document:    `"Aadhya"`,
			expectedErr: "something went wrong",
		},
		{
			name:     "conditional error() leaves other input untouched",
			path:     `if . == "Aadhya" then error("something went wrong") else . end`,
			document: `"Aaliyah"`,
			expected: "Aaliyah",
		},
		{
			name:        "runtime type error is propagated",
			path:        `. + 1`,
			document:    `"Aadhya"`,
			expectedErr: "cannot add",
		},
		{
			name:     "no match yields no result and no error",
			path:     `.[] | select(. == "nope")`,
			document: `["Aadhya"]`,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mustParse(t, tt.path).Execute([]byte(tt.document))

			if tt.expectedErr != `` {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, result)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
