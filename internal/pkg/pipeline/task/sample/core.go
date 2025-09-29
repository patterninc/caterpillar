package sample

import (
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type core struct {
	sendRecord func(*record.Record, chan<- *record.Record)
}
