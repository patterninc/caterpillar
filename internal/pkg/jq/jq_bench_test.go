package jq

import (
	"encoding/json"
	"testing"
)

func BenchmarkQueryExecute(b *testing.B) {

	document := []byte(`{"id":7,"user":{"name":"Ann","email":"ann@example.com"},"items":[{"sku":"a","qty":1},{"sku":"b","qty":3}]}`)

	cases := []struct {
		name string
		expr string
	}{
		{name: "field", expr: `.user.name`},
		{name: "projection", expr: `{id, email: .user.email, total: ([.items[].qty] | add)}`},
		{name: "explode", expr: `.items[]`},
		{name: "sha256", expr: `.user.email | sha256`},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			raw, _ := json.Marshal(c.expr)
			q := &Query{}
			if err := json.Unmarshal(raw, q); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := q.Execute(document); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

}
