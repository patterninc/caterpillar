package converter

import (
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/objstorage/objstorageprovider"
	"github.com/cockroachdb/pebble/sstable"
	"github.com/cockroachdb/pebble/vfs"
)

type sst struct{}

const (
	sstTablePrefix = "caterpillar-*.sst"
)

func (c *sst) convert(data []byte, d string) ([]converterOutput, error) {
	lines := strings.Split(string(data), "\n")
	values := map[string]string{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, d, 2)
		if len(kv) != 2 {
			return nil, errors.New("invalid input for sst converter: expected 'key" + d + "value' format (got: " + line + ")")
		}
		values[kv[0]] = kv[1]
	}

	fileName, err := c.createSST(values)
	if err != nil {
		return nil, err
	}
	defer os.Remove(fileName)

	sstData, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	return []converterOutput{{Data: sstData}}, nil
}

func (c *sst) createSST(kvs map[string]string) (string, error) {
	tmpfile, err := os.CreateTemp("", sstTablePrefix)
	if err != nil {
		return "", err
	}

	f, err := vfs.Default.Create(tmpfile.Name())
	if err != nil {
		return "", err
	}
	defer f.Close()
	writable := objstorageprovider.NewFileWritable(f)
	w := sstable.NewWriter(writable, sstable.WriterOptions{
		Comparer:    pebble.DefaultComparer,
		Compression: sstable.SnappyCompression,
	})

	defer w.Close()
	// Keys must be in strictly increasing order and unique.
	keys := make([]string, 0, len(kvs))
	for k := range kvs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		ik := pebble.InternalKey{
			UserKey: []byte(k),
			Trailer: uint64(pebble.InternalKeyKindSet),
		}
		if err := w.Add(ik, []byte(kvs[k])); err != nil {
			return "", err
		}
	}

	if err := f.Sync(); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}
