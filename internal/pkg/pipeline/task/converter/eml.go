package converter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/jhillyerd/enmime"
)

type eml struct{}

const maxFilenameLength = 200

func (c *eml) convert(data []byte, _ string) ([]converterOutput, error) {

	envelope, err := enmime.ReadEnvelope(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse EML: %v", err)
	}

	var outputs []converterOutput

	if envelope.HTML != "" {
		if out := c.processOutput([]byte(envelope.HTML), "body.html", "text/html"); out != nil {
			outputs = append(outputs, *out)
		}
	}
	if envelope.Text != "" {
		if out := c.processOutput([]byte(envelope.Text), "body.txt", "text/plain"); out != nil {
			outputs = append(outputs, *out)
		}
	}

	// Extract headers
	headerMap := make(map[string]string)
	for _, key := range envelope.GetHeaderKeys() {
		headerMap[key] = envelope.GetHeader(key)
	}
	if len(headerMap) > 0 {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(headerMap); err == nil {
			if out := c.processOutput(buf.Bytes(), "headers.json", "application/json"); out != nil {
				outputs = append(outputs, *out)
			}
		}
	}

	for _, attachment := range envelope.Attachments {
		if out := c.processOutput(attachment.Content, attachment.FileName, attachment.ContentType); out != nil {
			outputs = append(outputs, *out)
		}
	}

	for _, inline := range envelope.Inlines {
		if out := c.processOutput(inline.Content, inline.FileName, inline.ContentType); out != nil {
			outputs = append(outputs, *out)
		}
	}

	return outputs, nil
}

func (c *eml) processOutput(content []byte, fileName string, contentType string) *converterOutput {
	if len(content) == 0 {
		return nil
	}

	fileName = filepath.Base(fileName)

	// Fallback for filename length
	if len(fileName) > maxFilenameLength {
		ext := filepath.Ext(fileName)
		base := fileName[:len(fileName)-len(ext)]
		base = base[:maxFilenameLength-len(ext)]
		fileName = base + ext
	}

	// Fallback for missing content type
	if contentType == "" {
		opts := mime.TypeByExtension(filepath.Ext(fileName))
		if opts != "" {
			contentType = opts
		} else {
			contentType = "application/octet-stream"
		}
	}

	return &converterOutput{
		Data: content,
		Metadata: map[string]string{
			"converter_filename": fileName,
			"content_type":       contentType,
		},
	}
}
