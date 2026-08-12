package sample

import (
	"crypto/rand"
	"math/big"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
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
	} else {
		ack.Release(row.Context)
	}

	return nil

}

func (r *random) drain(output chan<- *record.Record) error {

	if l := int64(len(r.buffer)); l > 0 {

		// draws are with replacement, so a buffered record can be sent zero, one, or
		// many times. Each send registers itself, so no tally is needed.
		for i := 0; i < r.limit; i++ {

			index, err := rand.Int(rand.Reader, big.NewInt(l))
			if err != nil {
				// bailing out mid-draw leaves the buffer unreleased otherwise
				for _, row := range r.buffer {
					ack.Reject(row.Context)
				}
				return err
			}

			r.sendRecord(r.buffer[index.Int64()], output)

		}

		for _, row := range r.buffer {
			ack.Release(row.Context)
		}

	}

	return nil

}
