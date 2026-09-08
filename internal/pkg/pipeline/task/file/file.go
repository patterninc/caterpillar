package file

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/textutil"
)

const (
	defaultPath            = `/tmp/caterpillar.txt`
	defaultRegion          = `us-west-2`
	defaultDelimiter       = "\n"
	defaultSuccessFileName = `_SUCCESS`
)

type reader interface {
	read(string) (io.ReadCloser, error)
	parse(string) ([]string, error)
}

var (
	ctx     = context.Background()
	readers = map[string]func(*file) (reader, error){
		s3Scheme:   newS3Reader,
		fileScheme: newLocalReader,
	}
	writers = map[string]func(*file, *record.Record, io.Reader) error{
		s3Scheme:   writeS3File,
		fileScheme: writeLocalFile,
	}
)

type file struct {
	task.Base       `yaml:",inline" json:",inline"`
	Path            config.String            `yaml:"path,omitempty" json:"path,omitempty"`
	SuccessFile     bool                     `yaml:"success_file,omitempty" json:"success_file,omitempty"`
	SuccessFileName config.String            `yaml:"success_file_name,omitempty" json:"success_file_name,omitempty"`
	Region          string                   `yaml:"region,omitempty" json:"region,omitempty"`
	StorageClass    storageClass             `yaml:"storage_class,omitempty" json:"storage_class,omitempty"`
	Tags            map[string]config.String `yaml:"tags,omitempty" json:"tags,omitempty"`
	Delimiter       string                   `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`

	// readOnce/readPaths/readErr/readReader/readIdx coordinate concurrent
	// readers under task_concurrency > 1: the pipeline runs Run() N times
	// concurrently on this same *file instance (see runTaskConcurrently), so
	// the glob is expanded exactly once and workers claim disjoint paths off
	// the shared slice via readIdx instead of each re-reading every file.
	readOnce   sync.Once
	readReader reader
	readPaths  []string
	readErr    error
	readIdx    atomic.Int64
}

func New() (task.Task, error) {
	ensureStorageClasses()
	return &file{
		Path:            defaultPath,
		Region:          defaultRegion,
		StorageClass:    defaultStorageClass,
		Delimiter:       defaultDelimiter,
		SuccessFileName: defaultSuccessFileName,
	}, nil
}

func (f *file) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	if err := validateStorageClass(f.StorageClass); err != nil {
		return err
	}

	// let's check if we read file or we write file...
	if input != nil && output != nil {
		return task.ErrPresentInputOutput
	}

	// do we send data to output?
	if input == nil {
		if err := f.readFile(output); err != nil {
			return err
		}
	} else {
		if err := f.writeFile(input); err != nil {
			return err
		}

		// do we need to write _SUCCESS file?
		if f.SuccessFile {
			if err := f.writeSuccessFile(); err != nil {
				return err
			}
		}
	}

	return nil

}

// readFile is invoked once per worker when task_concurrency > 1 (the pipeline
// calls Run this many times concurrently on this same *file instance, sharing
// the output channel — see runTaskConcurrently). The glob is only ever
// expanded once, on whichever worker gets there first; every worker then
// claims disjoint indices out of the shared path list via readIdx, so N
// workers split the file list N ways instead of each reading every file.
func (f *file) readFile(output chan<- *record.Record) error {

	f.readOnce.Do(func() {
		f.readReader, f.readPaths, f.readErr = f.newReader()
	})

	if f.readErr != nil {
		return f.readErr
	}

	for {

		idx := f.readIdx.Add(1) - 1
		if idx >= int64(len(f.readPaths)) {
			break
		}

		if err := f.readPath(f.readPaths[idx], output); err != nil {
			return err
		}

	}

	return nil

}

// newReader resolves f.Path's glob into a scheme-appropriate reader and the
// full list of matched paths. Called at most once per file instance, guarded
// by readOnce in readFile.
func (f *file) newReader() (reader, []string, error) {

	// let's get the glob
	glob, err := f.Path.Get(nil)
	if err != nil {
		return nil, nil, err
	}

	// Determine the scheme from the path
	parsedURL, err := url.Parse(glob)
	if err != nil {
		return nil, nil, err
	}
	pathScheme := parsedURL.Scheme
	if pathScheme == `` {
		pathScheme = fileScheme
	}

	newReaderFunction, found := readers[pathScheme]
	if !found {
		return nil, nil, unknownSchemeError(pathScheme)
	}

	// let's create a reader
	rdr, err := newReaderFunction(f)
	if err != nil {
		return nil, nil, err
	}

	// let's parse the glob to get all paths
	paths, err := rdr.parse(glob)
	if err != nil {
		return nil, nil, err
	}

	return rdr, paths, nil

}

// readPath reads a single matched path and sends its content to output.
func (f *file) readPath(path string, output chan<- *record.Record) error {

	readerCloser, err := f.readReader.read(path)
	if err != nil {
		return err
	}
	defer readerCloser.Close()

	content, err := io.ReadAll(readerCloser)
	if err != nil {
		return err
	}

	// Create a default record with context
	fileName := textutil.SlugifyFileName(filepath.Base(path))
	rc := &record.Record{Context: ctx}
	rc.SetContextValue(string(task.CtxKeyFileNameWrite), fileName)
	rc.SetContextValue(string(task.CtxKeyFilePathWrite), textutil.SlugifyFilePath(path))

	// let's write content to output channel
	f.SendData(rc.Context, content, output)

	return nil

}

// abort settles rc as failed, then returns err, so a source deferring
// acknowledgement redelivers the record rather than waiting forever for a
// write that will never happen.
//
// Only rc is rejected, never the records queued behind it: sibling workers are
// still running and will write those, and the pipeline rejects whatever is
// genuinely left over once every worker has returned.
func (f *file) abort(rc *record.Record, err error) error {

	ack.Reject(rc.Context)

	return err

}

func (f *file) writeFile(input <-chan *record.Record) error {

	for {
		rc, ok := f.GetRecord(input)
		if !ok {
			break
		}

		// Evaluate the path with the record context
		path, err := f.Path.Get(rc)
		if err != nil {
			return f.abort(rc, err)
		}

		// Determine the scheme from the evaluated path
		parsedURL, err := url.Parse(path)
		if err != nil {
			return f.abort(rc, err)
		}
		pathScheme := parsedURL.Scheme
		if pathScheme == `` {
			pathScheme = fileScheme
		}

		// only the fields writerFunction reads are copied here — f itself
		// carries the sync.Once/atomic read-concurrency state (and the
		// embedded task.Base mutex), which must never be struct-copied.
		fs := &file{
			Path:            f.Path,
			SuccessFile:     f.SuccessFile,
			SuccessFileName: f.SuccessFileName,
			Region:          f.Region,
			StorageClass:    f.StorageClass,
			Tags:            f.Tags,
			Delimiter:       f.Delimiter,
		}

		filePath, found := rc.GetContextValue(string(task.CtxKeyArchiveFileNameWrite))
		if found {
			if filePath == "" {
				log.Fatal("required file path")
			}

			filePath = strings.ReplaceAll(filePath, "\\", "/")

			fs.Path = f.Path + config.String(filePath)
		}

		writerFunction, found := writers[pathScheme]
		if !found {
			return f.abort(rc, unknownSchemeError(pathScheme))
		}
		if err := writerFunction(fs, rc, bytes.NewReader(rc.Data)); err != nil {
			return f.abort(rc, err)
		}

		ack.Release(rc.Context)
	}

	return nil

}

func (f *file) writeSuccessFile() error {

	successFileName, err := f.SuccessFileName.Get(nil)
	if err != nil {
		return err
	}

	path, err := f.Path.Get(nil)
	if err != nil {
		return err
	}

	if i := strings.LastIndex(path, "/"); i >= 0 {
		successFileName = path[0:i+1] + successFileName
	}

	// Determine the scheme from the path
	parsedURL, err := url.Parse(path)
	if err != nil {
		return err
	}
	pathScheme := parsedURL.Scheme
	if pathScheme == `` {
		pathScheme = fileScheme
	}

	writerFunction, found := writers[pathScheme]
	if !found {
		return unknownSchemeError(pathScheme)
	}

	successFile := &file{
		Path:         config.String(successFileName),
		Region:       f.Region,
		StorageClass: f.StorageClass,
		Tags:         f.Tags,
	}

	return writerFunction(successFile, nil, bytes.NewReader([]byte{}))

}

func unknownSchemeError(scheme string) error {
	return fmt.Errorf("unknown scheme: %s", scheme)
}
