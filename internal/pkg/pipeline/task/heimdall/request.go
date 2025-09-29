package heimdall

type jobRequest struct {
	Name            string         `yaml:"name,omitempty" json:"name,omitempty"`
	Version         string         `yaml:"version,omitempty" json:"version,omitempty"`
	Context         map[string]any `yaml:"context,omitempty" json:"context,omitempty"`
	CommandCriteria []string       `yaml:"command_criteria,omitempty" json:"command_criteria"`
	ClusterCriteria []string       `yaml:"cluster_criteria,omitempty" json:"cluster_criteria"`
	Tags            []string       `yaml:"tags,omitempty" json:"tags,omitempty"`
}
