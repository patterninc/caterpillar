package file

import (
	"io"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar"
)

const (
	fileScheme       = `file`
	filePrefix       = fileScheme + `://`
	filePrefixLength = len(filePrefix)
)

type localReader struct{}

var lclReader = localReader{}

func newLocalReader(f *file) (reader, error) {
	return &lclReader, nil
}

func (r *localReader) read(path string) (io.ReadCloser, error) {

	inputFile, err := os.Open(path[getPathIndex(path):])
	if err != nil {
		return nil, err
	}

	return inputFile, nil

}

func (r *localReader) parse(glob string) ([]string, error) {

	paths, err := doublestar.Glob(glob)
	if err != nil {
		return nil, err
	}

	return paths, nil

}

func writeLocalFile(f *file, reader io.Reader) error {

	path, err := f.Path.Get(f.CurrentRecord)
	if err != nil {
		return err
	}

	outputFile, err := os.Create((path[getPathIndex(path):]))
	if err != nil {
		return err
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, reader)
	if err != nil {
		return err
	}

	return nil

}

func getPathIndex(path string) int {

	// let's figure out if we need to trim filePrefix
	index := strings.Index(path, filePrefix)
	if index == 0 {
		index += filePrefixLength
	} else {
		index = 0
	}

	return index

}
