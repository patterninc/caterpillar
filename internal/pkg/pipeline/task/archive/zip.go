package archive

import (
	"archive/zip"
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

type zipArchive struct {
	*task.Base
	*channelStruct
}

func (z *zipArchive) Read() {
	for {
		rc, ok := z.GetRecord(z.InputChan)
		if !ok {
			break
		}

		if len(rc.Data) == 0 {
			ack.Drop(rc.Context)
			continue
		}

		b := rc.Data

		r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			log.Fatal(err)
		}

		// fan-out: the ack must cover every file before any of them is sent,
		// or a downstream Done/Fail for the first could race ahead of a later
		// count adjustment.
		regularFiles := 0
		for _, f := range r.File {
			if f.FileInfo().Mode().IsRegular() {
				regularFiles++
			}
		}
		ack.Fanout(rc.Context, regularFiles)

		for _, f := range r.File {

			// check the file type is regular file
			if f.FileInfo().Mode().IsRegular() {

				rc.SetContextValue(string(task.CtxKeyArchiveFileNameWrite), textutil.SlugifyFileName(filepath.Base(f.Name)))

				fs, err := f.Open()
				if err != nil {
					log.Fatal(err)
				}

				buf := make([]byte, f.FileInfo().Size())

				_, err = io.ReadFull(fs, buf)
				fs.Close()
				if err != nil && err != io.EOF {
					log.Fatal(err)
				}

				z.SendData(rc.Context, buf, z.OutputChan)
			}
		}
	}
}

func (z *zipArchive) Write() {

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)
	var rc record.Record
	var ctxs []context.Context

	for {
		rec, ok := z.GetRecord(z.InputChan)
		if !ok {
			break
		}

		filePath, found := rec.GetContextValue(string(task.CtxKeyFileNameWrite))
		if !found {
			log.Fatal("filepath not set in context")
		}

		if filePath == "" {
			log.Fatal("empty filepath in context")
		}

		filePath = strings.ReplaceAll(filePath, "\\", "/")

		w, err := zipWriter.Create(filePath)
		if err != nil {
			log.Fatal(err)
		}
		_, err = w.Write(rec.Data)
		if err != nil {
			log.Fatal(err)
		}

		rc.Context = rec.Context
		ctxs = append(ctxs, rec.Context)
	}

	if err := zipWriter.Close(); err != nil {
		log.Fatal(err)
	}

	// fan-in: the archive record is produced from every input consumed above,
	// so its ack must transitively complete all of theirs.
	joinedAck := ack.Joined(ctxs...)

	// Send the complete ZIP archive
	z.SendData(ack.WithContext(rc.Context, joinedAck), zipBuf.Bytes(), z.OutputChan)

}
