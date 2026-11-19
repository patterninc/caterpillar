package converter

import (
	"testing"
)

func BenchmarkCSVConvert(b *testing.B) {

	row := []byte(`Ann,30,1000.50,"Engineering, Platform"`)

	cases := []struct {
		name    string
		columns []*csvColumn
	}{
		{name: "strings", columns: []*csvColumn{{Name: `name`}, {Name: `age`}, {Name: `salary`}, {Name: `department`}}},
		{name: "numeric", columns: []*csvColumn{{Name: `name`}, {Name: `age`, IsNumeric: true}, {Name: `salary`, IsNumeric: true}, {Name: `department`}}},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			conv := &csv{Columns: c.columns}
			b.ReportAllocs()
			b.SetBytes(int64(len(row)))
			for i := 0; i < b.N; i++ {
				if _, err := conv.convert(row, ``); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

}
