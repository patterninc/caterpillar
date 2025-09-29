package sample

import (
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type sampler interface {
	filter(*record.Record, chan<- *record.Record) error
	drain(chan<- *record.Record) error
}
