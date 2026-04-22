package profile

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const s3Scheme = `s3`

// Dump enables file-based profiling. CPU profiling runs until the returned
// closure is called, which then writes heap, goroutine, block, and mutex
// snapshots under dest. dest may be a local directory or an s3://bucket/prefix
// URI. The closure is idempotent.
func Dump(dest string) (func(), error) {

	sink, err := newSink(dest)
	if err != nil {
		return nil, err
	}

	// Block and mutex profiles require sampling to be enabled at runtime; only
	// turn them on when the caller asked for a dump since they add overhead.
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)

	cpuWriter, err := sink.writer(`cpu.pprof`)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(cpuWriter); err != nil {
		cpuWriter.Close()
		return nil, err
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			pprof.StopCPUProfile()
			if err := cpuWriter.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "profile cpu: %s\n", err)
			}
			for _, name := range []string{`heap`, `goroutine`, `block`, `mutex`} {
				writeNamed(sink, name)
			}
		})
	}, nil

}

// Serve starts a net/http/pprof server on addr in a goroutine. Listener
// errors are logged to stderr but do not affect the caller.
func Serve(addr string) {

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			fmt.Fprintf(os.Stderr, "profile-server: %s\n", err)
		}
	}()

}

func writeNamed(s sink, name string) {

	p := pprof.Lookup(name)
	if p == nil {
		return
	}

	w, err := s.writer(name + `.pprof`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "profile %s: %s\n", name, err)
		return
	}
	defer w.Close()

	if err := p.WriteTo(w, 0); err != nil {
		fmt.Fprintf(os.Stderr, "profile %s: %s\n", name, err)
	}

}

// sink abstracts the destination of pprof files. Local sinks stream directly
// to disk; S3 sinks buffer per file and upload on Close.
type sink interface {
	writer(name string) (io.WriteCloser, error)
}

func newSink(dest string) (sink, error) {

	u, err := url.Parse(dest)
	if err != nil {
		return nil, err
	}

	if u.Scheme == s3Scheme {
		ctx := context.Background()
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, err
		}
		return &s3Sink{
			ctx:    ctx,
			client: s3.NewFromConfig(awsCfg),
			bucket: u.Host,
			prefix: strings.TrimPrefix(u.Path, `/`),
		}, nil
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	return &localSink{dir: dest}, nil

}

type localSink struct {
	dir string
}

func (s *localSink) writer(name string) (io.WriteCloser, error) {
	return os.Create(filepath.Join(s.dir, name))
}

type s3Sink struct {
	ctx    context.Context
	client *s3.Client
	bucket string
	prefix string
}

func (s *s3Sink) writer(name string) (io.WriteCloser, error) {
	return &s3Writer{
		ctx:    s.ctx,
		client: s.client,
		bucket: s.bucket,
		key:    path.Join(s.prefix, name),
		buf:    &bytes.Buffer{},
	}, nil
}

type s3Writer struct {
	ctx    context.Context
	client *s3.Client
	bucket string
	key    string
	buf    *bytes.Buffer
}

func (w *s3Writer) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *s3Writer) Close() error {
	_, err := w.client.PutObject(w.ctx, &s3.PutObjectInput{
		Bucket: &w.bucket,
		Key:    &w.key,
		Body:   bytes.NewReader(w.buf.Bytes()),
	})
	return err
}
