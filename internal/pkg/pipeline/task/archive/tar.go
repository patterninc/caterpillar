package archive

import (
	"archive/tar"
	"bytes"
	"io"
	"log"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type tarArchive struct {
	*task.Base
	FileName   string
	Record     *record.Record
	OutputChan chan<- *record.Record
}

func (t *tarArchive) Read(b []byte) {
	r := tar.NewReader(bytes.NewReader(b))

	for {
		header, err := r.Next()
		if err != nil {
			break
		}

		// check the file type is regular file
		if header.Typeflag == tar.TypeReg {
			buf := make([]byte, header.Size)
			if _, err := io.ReadFull(r, buf); err != nil && err != io.EOF {
				log.Fatal(err)
			}
			t.SendData(t.Record.Context, buf, t.OutputChan)
		}

	}
}

func (t *tarArchive) Write(b []byte) {

	if t.FileName == "" {
		log.Fatal("file name is required to create tar archive")
	}

	tw := tar.NewWriter(bytes.NewBuffer(b))

	tw.WriteHeader(
		&tar.Header{Name: t.FileName,
			Mode: 0777,
			Size: int64(len(b))})

	_, err := tw.Write(b)
	if err != nil {
		log.Fatal(err)
	}

	err = tw.Close()
	if err != nil {
		log.Fatal(err)
	}

	t.SendData(t.Record.Context, b, t.OutputChan)
}
