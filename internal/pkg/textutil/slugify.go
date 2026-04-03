package textutil

import (
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]+`)

const maxFilenameLength = 200

// Slugify replaces non-alphanumeric characters with underscores, trims
// leading/trailing underscores, and lowercases the result.
// e.g. "Report (1)" -> "report_1"
func Slugify(name string) string {
	result := nonAlphanumeric.ReplaceAllString(name, "_")
	result = strings.Trim(result, "_")
	return strings.ToLower(result)
}

// SlugifyFileName slugifies a filename while preserving the extension.
// The stem is slugified via Slugify and the extension is lowercased.
// Empty stems default to "file". Filenames exceeding 200 characters
// are truncated.
// e.g. "Report (1).CSV" -> "report_1.csv"
func SlugifyFileName(name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	sanitized := Slugify(stem)
	if sanitized == "" {
		sanitized = "file"
	}
	ext = strings.ToLower(ext)
	fileName := sanitized + ext

	if len(fileName) > maxFilenameLength {
		base := sanitized[:maxFilenameLength-len(ext)]
		base = strings.TrimRight(base, "_")
		fileName = base + ext
	}

	return fileName
}
