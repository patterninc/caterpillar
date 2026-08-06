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
		ack.Drop(row.Context)
	}

	return nil

}

func (r *random) drain(output chan<- *record.Record) error {

	if l := int64(len(r.buffer)); l > 0 {

		// draws are with replacement, so a buffered record can be sent zero,
		// one, or many times. Tally every draw and size each ack for its final
		// send count before sending any, or a downstream Done/Fail for an
		// earlier send could race ahead of a later AddBranch on the same ack.
		counts := make([]int, l)
		for i := 0; i < r.limit; i++ {

			index, err := rand.Int(rand.Reader, big.NewInt(l))
			if err != nil {
				return err
			}

			counts[index.Int64()]++

		}

		for i, count := range counts {
			ack.Fanout(r.buffer[i].Context, count)
		}

		for i, count := range counts {
			for range count {
				r.sendRecord(r.buffer[i], output)
			}
		}

	}

	return nil

}
