package archive

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/textutil"
)

type tarArchive struct {
	*task.Base
	*channelStruct
}

// tarFile is a regular file extracted from an archive, buffered until the
// total file count is known and the fan-out ack can be sized.
type tarFile struct {
	name string
	data []byte
}

func (t *tarArchive) Read() {

	for {
		rc, ok := t.GetRecord(t.InputChan)
		if !ok {
			break
		}

		if len(rc.Data) == 0 {
			ack.Drop(rc.Context)
			continue
		}

		b := rc.Data

		// fan-out: the ack must cover every file before any of them is sent,
		// or a downstream Done/Fail for the first could race ahead of a later
		// count adjustment. tar readers are forward-only, so extract in a
		// single pass and send once the count is known.
		files := make([]tarFile, 0)

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
			if header.Typeflag != tar.TypeReg {
				continue
			}

			buf := make([]byte, header.Size)
			if _, err := io.ReadFull(r, buf); err != nil && err != io.EOF {
				log.Fatal(err)
			}

			files = append(files, tarFile{
				name: textutil.SlugifyFileName(filepath.Base(header.Name)),
				data: buf,
			})
		}

		ack.Fanout(rc.Context, len(files))

		for _, f := range files {
			rc.SetContextValue(string(task.CtxKeyArchiveFileNameWrite), f.name)
			t.SendData(rc.Context, f.data, t.OutputChan)
		}
	}
}

func (t *tarArchive) Write() {

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var rc record.Record
	var ctxs []context.Context

	for {
		rec, ok := t.GetRecord(t.InputChan)
		if !ok {
			break
		}
		ctxs = append(ctxs, rec.Context)

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

	// fan-in: the archive record is produced from every input consumed above,
	// so its ack must transitively complete all of theirs.
	joinedAck := ack.Joined(ctxs...)

	t.SendData(ack.WithContext(rc.Context, joinedAck), buf.Bytes(), t.OutputChan)
}
