package sample

import (
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type tail struct {
	core
	limit  int
	count  int
	index  int
	buffer []*record.Record
}

func newTail(s *sample) (sampler, error) {
	return &tail{
		core: core{
			sendRecord: s.SendRecord,
		},
		limit:  s.Limit,
		buffer: make([]*record.Record, s.Limit),
	}, nil
}

func (t *tail) filter(r *record.Record, _ chan<- *record.Record) error {

	t.buffer[t.index] = r
	t.index = (t.index + 1) % t.limit
	t.count++

	return nil

}

func (t *tail) drain(output chan<- *record.Record) error {

	start := t.index

	if t.count < t.limit {
		start = 0
		t.limit = t.count
	}

	for i := 0; i < t.limit; i++ {
		t.sendRecord(t.buffer[(start+i)%len(t.buffer)], output)
	}

	return nil

}
