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

// SlugifyFilePath slugifies each segment of a path while preserving the
// "/" separator between segments. A leading "<scheme>://<host>/" prefix
// (e.g. "s3://my-bucket/") is stripped so only the path within the source
// location remains; what remains is split on "/", interior segments are
// slugified via Slugify, and the final segment is slugified via
// SlugifyFileName so its extension is preserved. Backslashes are
// normalized to forward slashes, and empty segments (leading "/", "//",
// interior segments that slugify to "") are dropped.
// e.g. "s3://my-bucket/ReportType=A/data 1.CSV" -> "reporttype_a/data_1.csv"
func SlugifyFilePath(path string) string {
	if path == "" {
		return ""
	}
	if i := strings.Index(path, "://"); i > 0 {
		rest := path[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			path = rest[j:]
		} else {
			path = ""
		}
	}
	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	nonEmpty := parts[:0]
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	out := make([]string, 0, len(nonEmpty))
	for i, p := range nonEmpty {
		if i == len(nonEmpty)-1 {
			out = append(out, SlugifyFileName(p))
			continue
		}
		if s := Slugify(p); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, "/")
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
