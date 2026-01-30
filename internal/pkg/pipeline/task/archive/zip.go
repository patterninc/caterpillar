package archive

import (
	"archive/zip"
	"bytes"
	"io"
	"log"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type zipArchive struct {
	*task.Base
	FileName   string
	Record     *record.Record
	OutputChan chan<- *record.Record
}

func (z *zipArchive) Read(b []byte) {
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range r.File {

		// check the file type is regular file
		if f.FileInfo().Mode().IsRegular() {

			rc, err := f.Open()
			if err != nil {
				log.Fatal(err)
			}

			buf := make([]byte, f.FileInfo().Size())

			_, err = rc.Read(buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}

			rc.Close()

			z.SendData(z.Record.Context, buf, z.OutputChan)
		}
	}
}

func (z *zipArchive) Write(b []byte) {

	zipBuf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(zipBuf)

	if z.FileName == "" {
		log.Fatal("file name is required to create zip archive")
	}

	w, _ := zipWriter.Create(z.FileName)
	w.Write(b)

	zipWriter.Close()

	z.SendData(z.Record.Context, zipBuf.Bytes(), z.OutputChan)

}
