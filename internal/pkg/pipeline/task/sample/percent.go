package sample

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

var (
	ErrOutOfRange = fmt.Errorf(`percent may be between 0 and 100 inclusive`)
)

type percent struct {
	core
	cutoff int
}

func newPercent(s *sample) (sampler, error) {

	if s.Percent < 0 || s.Percent > 100 {
		return nil, ErrOutOfRange
	}

	return &percent{
		core: core{
			sendRecord: s.SendRecord,
		},
		cutoff: s.Percent,
	}, nil

}

func (p *percent) filter(r *record.Record, output chan<- *record.Record) error {

	// Generate a secure random number between 0 and 100 inclusive
	n, err := rand.Int(rand.Reader, big.NewInt(101))
	if err != nil {
		return err
	}

	if n.Int64() < int64(p.cutoff) {
		p.sendRecord(r, output)
	}

	return nil

}

func (p *percent) drain(_ chan<- *record.Record) error {
	return nil
}
