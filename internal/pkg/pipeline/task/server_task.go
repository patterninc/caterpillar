package task

import (
	"github.com/patterninc/caterpillar/internal/pkg/duration"
)

type ServerBase struct {
	Base `yaml:",inline" json:",inline"`

	EndAfter duration.Duration `yaml:"end_after,omitempty" json:"end_after,omitempty"`
}
