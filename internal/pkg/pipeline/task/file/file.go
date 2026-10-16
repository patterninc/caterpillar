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
	"sync/atomic"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/textutil"
	"golang.org/x/sync/errgroup"
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

	// recordSeq is shared by read workers so record ids stay unique.
	recordSeq *atomic.Int64
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

func (f *file) readFile(output chan<- *record.Record) error {

	// let's get the glob
	glob, err := f.Path.Get(nil)
	if err != nil {
		return err
	}

	// Determine the scheme from the path
	parsedURL, err := url.Parse(glob)
	if err != nil {
		return err
	}
	pathScheme := parsedURL.Scheme
	if pathScheme == `` {
		pathScheme = fileScheme
	}

	newReaderFunction, found := readers[pathScheme]
	if !found {
		return unknownSchemeError(pathScheme)
	}

	// let's create a reader
	reader, err := newReaderFunction(f)
	if err != nil {
		return err
	}

	// let's parse the glob to get all paths
	paths, err := reader.parse(glob)
	if err != nil {
		return err
	}

	// if no paths are found, return nil
	if len(paths) == 0 {
		return nil
	}

	// One task instance per worker. SendData locks the task for the whole
	// context evaluation, so workers that share f wait on each other.
	// Workers pull paths from one queue, so a large file does not pin the
	// rest of a fixed list, and each path is still read once.
	f.recordSeq = &atomic.Int64{}
	workers := min(f.GetTaskConcurrency(), len(paths))
	g, gctx := errgroup.WithContext(context.Background())
	pathsCh := make(chan string)

	g.Go(func() error {
		defer close(pathsCh)
		for _, path := range paths {
			select {
			case pathsCh <- path:
			case <-gctx.Done():
				return gctx.Err()
			}
		}
		return nil
	})

	for range workers {
		worker := f.readWorker()
		g.Go(func() error {
			for path := range pathsCh {
				if err := worker.emitFile(reader, path, output); err != nil {
					return err
				}
			}
			return nil
		})
	}

	return g.Wait()

}

// readWorker is a copy of f with its own lock. Fields are listed by hand
// because Base embeds a mutex (a struct copy trips copylocks). Keep this
// list in sync with file and task.Base: a field left out is dropped for
// read workers, and nothing else checks it.
func (f *file) readWorker() *file {

	return &file{
		Base: task.Base{
			Name:            f.Name,
			Type:            f.Type,
			FailOnError:     f.FailOnError,
			TaskConcurrency: f.TaskConcurrency,
			Context:         f.Context,
		},
		Path:            f.Path,
		SuccessFile:     f.SuccessFile,
		SuccessFileName: f.SuccessFileName,
		Region:          f.Region,
		StorageClass:    f.StorageClass,
		Tags:            f.Tags,
		Delimiter:       f.Delimiter,
		recordSeq:       f.recordSeq,
	}

}

func (f *file) emitFile(r reader, path string, output chan<- *record.Record) error {

	readerCloser, err := r.read(path)
	if err != nil {
		return err
	}
	defer readerCloser.Close()

	content, err := io.ReadAll(readerCloser)
	if err != nil {
		return err
	}

	fileName := textutil.SlugifyFileName(filepath.Base(path))
	rc := &record.Record{Context: ctx}
	rc.SetContextValue(string(task.CtxKeyFileNameWrite), fileName)
	rc.SetContextValue(string(task.CtxKeyFilePathWrite), textutil.SlugifyFilePath(path))

	f.SendData(rc.Context, content, output)

	return nil

}

// SendData uses the shared counter when this task is a read worker. Each
// worker still locks only itself around context evaluation.
func (f *file) SendData(ctx context.Context, data []byte, output chan<- *record.Record) {

	if f.recordSeq == nil {
		f.Base.SendData(ctx, data, output)
		return
	}

	f.Lock()
	defer f.Unlock()

	f.SendRecord(&record.Record{
		ID:      int(f.recordSeq.Add(1)),
		Origin:  f.Name,
		Data:    data,
		Context: ctx,
	}, output)

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

		var fs file

		fs = *f
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
		if err := writerFunction(&fs, rc, bytes.NewReader(rc.Data)); err != nil {
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
