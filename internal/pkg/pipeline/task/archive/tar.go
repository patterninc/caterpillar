package archive

import (
	"archive/tar"
	"bytes"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type tarArchive struct {
	*task.Base
	*channelStruct
}

func (t *tarArchive) Read() {

	for {
		rc, ok := t.GetRecord(t.InputChan)
		if !ok {
			break
		}

		if len(rc.Data) == 0 {
			continue
		}

		b := rc.Data

		r := tar.NewReader(bytes.NewReader(b))

		for {
			header, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}

			// check the file type is regular file
			if header.Typeflag == tar.TypeReg {
				buf := make([]byte, header.Size)
				if _, err := io.ReadFull(r, buf); err != nil && err != io.EOF {
					log.Fatal(err)
				}
				rc.SetContextValue(string(task.CtxKeyArchiveFileNameWrite), filepath.Base(header.Name))
				t.SendData(rc.Context, buf, t.OutputChan)
			}

		}
	}
}

func (t *tarArchive) Write() {

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var rc record.Record

	for {
		rec, ok := t.GetRecord(t.InputChan)
		if !ok {
			break
		}
		b := rec.Data

		if len(b) == 0 {
			continue
		}

		filePath, found := rec.GetContextValue(string(task.CtxKeyFileNameWrite))
		if !found {
			log.Fatal("filepath not set in context")
		}

		if filePath == "" {
			log.Fatal("empty filepath in context")
		}

		filePath = strings.ReplaceAll(filePath, "\\", "/")

		header := &tar.Header{
			Name: filePath,
			Mode: 0600,
			Size: int64(len(b)),
		}
		if err := tw.WriteHeader(header); err != nil {
			log.Fatal(err)
		}

		if _, err := tw.Write(b); err != nil {
			log.Fatal(err)
		}

		rc.Context = rec.Context
	}

	if err := tw.Close(); err != nil {
		log.Fatal(err)
	}

	t.SendData(rc.Context, buf.Bytes(), t.OutputChan)
}
