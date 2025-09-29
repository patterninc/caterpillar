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

func (m Record) MarshalJSON() ([]byte, error) {
    type Alias Record // Prevent recursion
    return json.Marshal(&struct {
        Data string `json:"data"`
        *Alias
    }{
        Data:  string(m.Data),
        Alias: (*Alias)(&m),
    })
}

func (r *Record) Bytes() []byte {

	data, _ := json.Marshal(r)
	return data

}
