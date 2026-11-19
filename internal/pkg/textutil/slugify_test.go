package textutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Empty", input: "", expected: ""},
		{name: "Already clean", input: "report", expected: "report"},
		{name: "Spaces and parens", input: "Report (1)", expected: "report_1"},
		{name: "Runs collapse", input: "a -- b", expected: "a_b"},
		{name: "Leading and trailing trimmed", input: "__a b__", expected: "a_b"},
		{name: "Unicode is replaced", input: "café résumé", expected: "caf_r_sum"},
		{name: "Only symbols", input: "!!!", expected: ""},
		{name: "Digits kept", input: "Q3 2024", expected: "q3_2024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Slugify(tt.input))
		})
	}

}

func TestSlugifyFileName(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Extension lowercased", input: "Report (1).CSV", expected: "report_1.csv"},
		{name: "No extension", input: "README", expected: "readme"},
		{name: "Only last extension preserved", input: "data.tar.gz", expected: "data_tar.gz"},
		{name: "Empty stem defaults to file", input: ".env", expected: "file.env"},
		{name: "Symbol stem defaults to file", input: "###.txt", expected: "file.txt"},
		{name: "Empty input", input: "", expected: "file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SlugifyFileName(tt.input))
		})
	}

	t.Run("Long names are truncated to 200 characters", func(t *testing.T) {
		got := SlugifyFileName(strings.Repeat("a", 300) + ".json")
		assert.Len(t, got, maxFilenameLength)
		assert.True(t, strings.HasSuffix(got, ".json"))
	})

	t.Run("Truncation does not leave a trailing underscore", func(t *testing.T) {
		stem := strings.Repeat("a", 194) + " b" + strings.Repeat("c", 50)
		got := SlugifyFileName(stem + ".json")
		assert.Equal(t, strings.Repeat("a", 194)+".json", got)
	})

}

// An extension longer than the length cap makes the truncation slice bound
// negative. Unskip once SlugifyFileName guards against it.
func TestSlugifyFileNameHugeExtension(t *testing.T) {

	t.Skip("known bug: SlugifyFileName panics when the extension exceeds 200 characters (internal/pkg/textutil/slugify.go:93)")

	assert.NotPanics(t, func() {
		SlugifyFileName("a." + strings.Repeat("x", 250))
	})

}

func TestSlugifyFilePath(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "Empty", input: "", expected: ""},
		{name: "S3 URL", input: "s3://my-bucket/ReportType=A/data 1.CSV", expected: "reporttype_a/data_1.csv"},
		{name: "Bucket only", input: "s3://bucket-only", expected: ""},
		{name: "Absolute local path", input: "/tmp/Out Dir/File.TXT", expected: "tmp/out_dir/file.txt"},
		{name: "Relative path", input: "a/b/c.json", expected: "a/b/c.json"},
		{name: "Backslashes normalised", input: `dir\Sub Dir\x.csv`, expected: "dir/sub_dir/x.csv"},
		{name: "Empty segments dropped", input: "a//b///c.txt", expected: "a/b/c.txt"},
		{name: "Symbol-only segment dropped", input: "a/!!!/c.txt", expected: "a/c.txt"},
		{name: "Only separators", input: "///", expected: ""},
		{name: "File scheme", input: "file:///var/log/App.log", expected: "var/log/app.log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SlugifyFilePath(tt.input))
		})
	}

}

func TestStripURLScheme(t *testing.T) {

	tests := []struct {
		input    string
		expected string
	}{
		{input: "s3://bucket/key", expected: "/key"},
		{input: "s3://bucket", expected: ""},
		{input: "/plain/path", expected: "/plain/path"},
		{input: "://weird", expected: "://weird"},
		{input: "https://host/a/b", expected: "/a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, stripURLScheme(tt.input))
		})
	}

}
