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

		// this is a fan-out: one archive record can expand into multiple
		// file records, so the ack must represent all of them - counted up
		// front, before any of them is sent - or a downstream Done/Fail for
		// the first file could race ahead of a later count adjustment. tar
		// readers are forward-only, so counting takes its own pass over a
		// fresh reader.
		regularFiles := 0
		for counter := tar.NewReader(bytes.NewReader(b)); ; {
			header, err := counter.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			if header.Typeflag == tar.TypeReg {
				regularFiles++
			}
		}
		ack.Fanout(rc.Context, regularFiles)

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
				rc.SetContextValue(string(task.CtxKeyArchiveFileNameWrite), textutil.SlugifyFileName(filepath.Base(header.Name)))
				t.SendData(rc.Context, buf, t.OutputChan)
			}

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

	// this is a fan-in: one archive record is produced from every input
	// record consumed above, so its ack must transitively complete all of
	// theirs instead of discarding all but the last.
	joinedAck := ack.Joined(ctxs...)

	t.SendData(ack.WithContext(rc.Context, joinedAck), buf.Bytes(), t.OutputChan)
}
