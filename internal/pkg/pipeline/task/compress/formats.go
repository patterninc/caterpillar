package compress

import (
	"compress/flate"
	"compress/gzip"
	"io"

	"github.com/golang/snappy"
)

type compressionFormat struct {
	NewReader func(io.Reader) (io.ReadCloser, error)
	NewWriter func(io.Writer) io.WriteCloser
}

var (
	formatHandlers = map[string]*compressionFormat{
		`gzip`: {
			NewReader: func(r io.Reader) (io.ReadCloser, error) {
				return gzip.NewReader(r)
			},
			NewWriter: func(w io.Writer) io.WriteCloser {
				return gzip.NewWriter(w)
			}},
		`snappy`: {
			NewReader: func(r io.Reader) (io.ReadCloser, error) {
				return io.NopCloser(snappy.NewReader(r)), nil
			},
			NewWriter: func(w io.Writer) io.WriteCloser {
				return snappy.NewBufferedWriter(w)
			}},
		`zip`: {
			NewReader: func(r io.Reader) (io.ReadCloser, error) {
				return flate.NewReader(r), nil
			},
			NewWriter: func(w io.Writer) io.WriteCloser {
				writer, _ := flate.NewWriter(w, flate.DefaultCompression)
				return writer
			}},
	}
)
