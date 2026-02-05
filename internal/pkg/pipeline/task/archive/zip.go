package archive

import (
	"archive/zip"
	"bytes"
	"io"
	"log"
	"path/filepath"
	"strings"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
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
			continue
		}

		b := rc.Data

		r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range r.File {

			// check the file type is regular file
			if f.FileInfo().Mode().IsRegular() {

				rc.SetContextValue(string(task.CtxKeyFilePathRead), filepath.Base(f.Name))

				fs, err := f.Open()
				if err != nil {
					log.Fatal(err)
				}

				buf := make([]byte, f.FileInfo().Size())

				_, err = fs.Read(buf)
				if err != nil && err != io.EOF {
					log.Fatal(err)
				}

				fs.Close()

				z.SendData(rc.Context, buf, z.OutputChan)
			}
		}
	}
}

func (z *zipArchive) Write() {

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)
	var rc record.Record

	for {
		rec, ok := z.GetRecord(z.InputChan)
		if !ok {
			break
		}

		filePath, found := rec.GetContextValue(string(task.CtxKeyFilePath))
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
	}

	if err := zipWriter.Close(); err != nil {
		log.Fatal(err)
	}

	// Send the complete ZIP archive
	z.SendData(rc.Context, zipBuf.Bytes(), z.OutputChan)

}
