package record

import (
	"encoding/json"
)

type Record struct {
	ID     int               `yaml:"id,omitempty" json:"id,omitempty"`
	Origin string            `yaml:"origin,omitempty" json:"origin,omitempty"`
	Data   []byte            `yaml:"data,omitempty" json:"data,omitempty"`
	Meta   map[string]string `yaml:"meta,omitempty" json:"meta,omitempty"`
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
