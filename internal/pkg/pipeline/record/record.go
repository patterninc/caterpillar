package record

import (
	"context"
	"encoding/json"
)

type Record struct {
	ID      int             `yaml:"id,omitempty" json:"id,omitempty"`
	Origin  string          `yaml:"origin,omitempty" json:"origin,omitempty"`
	Data    []byte          `yaml:"data,omitempty" json:"data,omitempty"`
	Context context.Context `yaml:"-" json:"-"`
}

func (r *Record) Bytes() []byte {

	data, _ := json.Marshal(r)
	return data

}
