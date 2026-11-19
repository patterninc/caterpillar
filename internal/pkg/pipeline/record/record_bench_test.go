package record

import (
	"encoding/json"
	"strings"
	"testing"
)

func BenchmarkRecordMarshalJSON(b *testing.B) {

	for _, size := range []int{64, 4096} {
		r := &Record{ID: 1, Origin: `source`, Data: []byte(strings.Repeat(`x`, size))}
		b.Run(sizeName(size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for i := 0; i < b.N; i++ {
				if _, err := json.Marshal(r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

}

func BenchmarkRecordContextValue(b *testing.B) {

	r := &Record{}
	for _, k := range []string{`a`, `b`, `c`, `d`} {
		r.SetContextValue(k, `value`)
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := r.GetContextValue(`a`); !ok {
			b.Fatal(`missing`)
		}
	}

}

func sizeName(size int) string {

	if size >= 1024 {
		return `4KiB`
	}

	return `64B`

}
