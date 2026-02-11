package record

import (
	"encoding/json"
	"maps"
)

type Record struct {
	ID     int               `yaml:"id,omitempty" json:"id,omitempty"`
	Origin string            `yaml:"origin,omitempty" json:"origin,omitempty"`
	Data   []byte            `yaml:"data,omitempty" json:"data,omitempty"`
	Meta   map[string]string `yaml:"context,omitempty" json:"context,omitempty"` // keeping json key as context for backward compatibility
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

// Clone creates a deep copy of the record to prevent shared references across parallel pipeline branches.
func (r *Record) Clone() *Record {
	if r == nil {
		return nil
	}

	newRec := &Record{
		ID:     r.ID,
		Origin: r.Origin,
	}

	if r.Data != nil {
		newRec.Data = make([]byte, len(r.Data))
		copy(newRec.Data, r.Data)
	}

	if r.Meta != nil {
		newRec.Meta = make(map[string]string, len(r.Meta))
		maps.Copy(newRec.Meta, r.Meta)
	}

	return newRec
}
