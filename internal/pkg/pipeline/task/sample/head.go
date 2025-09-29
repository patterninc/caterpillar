package sample

import (
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type head struct {
	core
	limit int
	index int
}

func newHead(s *sample) (sampler, error) {
	return &head{
		core: core{
			sendRecord: s.SendRecord,
		},
		limit: s.Limit,
	}, nil
}

func (h *head) filter(r *record.Record, output chan<- *record.Record) error {

	if h.index < h.limit {
		h.sendRecord(r, output)
		h.index++
	}

	return nil

}

func (h *head) drain(_ chan<- *record.Record) error {
	return nil
}
