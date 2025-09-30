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

func getLocalReader(f *file, path string) (io.ReadCloser, error) {

	inputFile, err := os.Open(path[f.getPathIndex(path):])
	if err != nil {
		return nil, err
	}

	return inputFile, nil

}

func writeLocalFile(f *file, reader io.Reader) error {

	path, err := f.Path.Get(f.CurrentRecord)
	if err != nil {
		return err
	}
	outputFile, err := os.Create((path[f.getPathIndex(path):]))
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

func (f *file) getPathIndex(path string) int {

	// let's figure out if we need to trim filePrefix
	index := strings.Index(path, filePrefix)
	if index == 0 {
		index += filePrefixLength
	} else {
		index = 0
	}

	return index
}

func getPathsFromGlob(f *file) ([]string, error) {

	glob, err := f.Path.Get(f.CurrentRecord)
	if err != nil {
		return nil, err
	}

	paths, err := doublestar.Glob(glob)
	if err != nil {
		return nil, err
	}

	return paths, nil
	
}
