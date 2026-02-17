package converter

import (
	"bytes"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/jhillyerd/enmime"
)

type eml struct{}

func (c *eml) convert(data []byte, _ string) ([]converterOutput, error) {

	envelope, err := enmime.ReadEnvelope(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse EML: %v", err)
	}

	var outputs []converterOutput

	addOutput := func(content []byte, fileName string, contentType string) {
		if len(content) == 0 {
			return
		}

		fileName = filepath.Base(fileName)

		// Fallback for filename length
		if len(fileName) > 200 {
			ext := filepath.Ext(fileName)
			base := fileName[:len(fileName)-len(ext)]
			base = base[:200-len(ext)]
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

		outputs = append(outputs, converterOutput{
			Data: content,
			Metadata: map[string]string{
				"filename":     fileName,
				"content_type": contentType,
			},
		})
	}

	if envelope.HTML != "" {
		addOutput([]byte(envelope.HTML), "body.html", "text/html")
	}
	if envelope.Text != "" {
		addOutput([]byte(envelope.Text), "body.txt", "text/plain")
	}

	for _, attachment := range envelope.Attachments {
		addOutput(attachment.Content, attachment.FileName, attachment.ContentType)
	}

	for _, inline := range envelope.Inlines {
		addOutput(inline.Content, inline.FileName, inline.ContentType)
	}

	return outputs, nil
}
