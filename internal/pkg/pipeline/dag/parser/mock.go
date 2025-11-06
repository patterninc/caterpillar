package parser

import "github.com/patterninc/caterpillar/internal/pkg/pipeline/record"

// MockTask is a simple task implementation for testing
type MockTask struct {
	name string
}

func (m *MockTask) GetName() string {
	return m.name
}

func (m *MockTask) Run(<-chan *record.Record, chan<- *record.Record) error {
	return nil
}

func (m *MockTask) GetInputCount() int {
	return 1
}

func (m *MockTask) GetFailOnError() bool {
	return false
}
