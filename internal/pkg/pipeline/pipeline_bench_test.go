package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

// benchSource emits a fixed number of small records, standing in for a file or
// queue source without touching disk or network.
type benchSource struct {
	task.Base
	records int
	payload []byte
}

func (s *benchSource) Run(_ <-chan *record.Record, output chan<- *record.Record) error {

	for i := 0; i < s.records; i++ {
		s.SendData(context.Background(), s.payload, output)
	}

	return nil

}

// BenchmarkLinearPipeline measures channel handoff through a chain of
// pass-through tasks, which is the per-record overhead every pipeline pays.
func BenchmarkLinearPipeline(b *testing.B) {

	const records = 1000

	for _, stages := range []int{1, 4} {
		b.Run(fmt.Sprintf("stages=%d", stages), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				p := &Pipeline{ChannelSize: 100}
				p.Tasks = append(p.Tasks, &benchSource{
					Base:    task.Base{Name: `source`},
					records: records,
					payload: []byte(`{"id":1,"name":"caterpillar"}`),
				})
				for s := 0; s < stages; s++ {
					p.Tasks = append(p.Tasks, &task.Base{Name: fmt.Sprintf("stage-%d", s)})
				}
				if err := p.Init(); err != nil {
					b.Fatal(err)
				}
				if err := p.Run(); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportMetric(float64(records*b.N)/b.Elapsed().Seconds(), `records/s`)
		})
	}

}
