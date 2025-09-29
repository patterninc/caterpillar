package sample

import (
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type nth struct {
	core
	divider int
	index   int
}

func newNth(s *sample) (sampler, error) {
	return &nth{
		core: core{
			sendRecord: s.SendRecord,
		},
		divider: s.Divider,
	}, nil
}

func (n *nth) filter(r *record.Record, output chan<- *record.Record) error {

	if n.index%n.divider == 0 {
		n.sendRecord(r, output)
	}

	n.index++

	return nil

}

func (n *nth) drain(_ chan<- *record.Record) error {
	return nil
}
