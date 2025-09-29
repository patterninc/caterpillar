package duration

import (
	"time"

	"gopkg.in/yaml.v3"
)

type Duration time.Duration

// UnmarshalYAML allows YAML like "duration: 15s"
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {

	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)

	return nil

}
