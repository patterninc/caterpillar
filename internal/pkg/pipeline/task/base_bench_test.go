package task

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

func BenchmarkSendRecord(b *testing.B) {

	payload := []byte(`{"id":42,"name":"caterpillar","tags":["a","b","c"]}`)

	withContext := &Base{}
	if err := yaml.Unmarshal([]byte("context:\n  id: '.data | fromjson | .id'"), withContext); err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name string
		base *Base
	}{
		{name: "no_context", base: &Base{}},
		{name: "jq_context", base: withContext},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			out := make(chan *record.Record, 1)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				c.base.SendRecord(&record.Record{ID: i, Data: payload, Context: context.Background()}, out)
				<-out
			}
		})
	}

}
