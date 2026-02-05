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

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
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
	Path            config.String `yaml:"path,omitempty" json:"path,omitempty"`
	SuccessFile     bool          `yaml:"success_file,omitempty" json:"success_file,omitempty"`
	SuccessFileName config.String `yaml:"success_file_name,omitempty" json:"success_file_name,omitempty"`
	Region          string        `yaml:"region,omitempty" json:"region,omitempty"`
	Delimiter       string        `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
}

func New() (task.Task, error) {
	return &file{
		Path:            defaultPath,
		Region:          defaultRegion,
		Delimiter:       defaultDelimiter,
		SuccessFileName: defaultSuccessFileName,
	}, nil
}

func (f *file) Run(input <-chan *record.Record, output chan<- *record.Record) error {

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

	for _, path := range paths {

		readerCloser, err := reader.read(path)
		if err != nil {
			return err
		}
		defer readerCloser.Close()

		content, err := io.ReadAll(readerCloser)
		if err != nil {
			return err
		}

		// Create a default record with context
		rc := &record.Record{Context: ctx}
		rc.SetContextValue(string(task.CtxKeyFileNameWrite), filepath.Base(path))

		// let's write content to output channel
		f.SendData(rc.Context, content, output)

	}

	return nil

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
			return err
		}

		// Determine the scheme from the evaluated path
		parsedURL, err := url.Parse(path)
		if err != nil {
			return err
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
			return unknownSchemeError(pathScheme)
		}
		if err := writerFunction(&fs, rc, bytes.NewReader(rc.Data)); err != nil {
			return err
		}
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
		Path:   config.String(successFileName),
		Region: f.Region,
	}

	return writerFunction(successFile, nil, bytes.NewReader([]byte{}))

}

func unknownSchemeError(scheme string) error {
	return fmt.Errorf("unknown scheme: %s", scheme)
}
