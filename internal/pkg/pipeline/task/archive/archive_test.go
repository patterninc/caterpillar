package archive

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnmarshalYAMLRejectsUnsupportedFormat(t *testing.T) {
	var archive core

	err := yaml.Unmarshal([]byte("format: rar\n"), &archive)
	if err == nil {
		t.Fatal("expected unsupported archive format to be rejected")
	}

	const want = "invalid format: rar (must be 'zip' or 'tar')"
	if err.Error() != want {
		t.Fatalf("unexpected error: got %q, want %q", err, want)
	}
}
