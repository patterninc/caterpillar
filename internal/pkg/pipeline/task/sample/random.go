package sample

import (
	"crypto/rand"
	"math/big"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type random struct {
	core
	limit  int
	size   int
	buffer []*record.Record
}

func newRandom(s *sample) (sampler, error) {
	return &random{
		core: core{
			sendRecord: s.SendRecord,
		},
		limit:  s.Limit,
		size:   s.Size,
		buffer: make([]*record.Record, 0, s.Size),
	}, nil
}

func (r *random) filter(row *record.Record, _ chan<- *record.Record) error {

	if len(r.buffer) < r.size {
		r.buffer = append(r.buffer, row)
	}

	return nil

}

func (r *random) drain(output chan<- *record.Record) error {

	if l := int64(len(r.buffer)); l > 0 {
		for i := 0; i < r.limit; i++ {

			index, err := rand.Int(rand.Reader, big.NewInt(l))
			if err != nil {
				return err
			}

			r.sendRecord(r.buffer[index.Int64()], output)

		}
	}

	return nil

}
