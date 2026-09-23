package file

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestReadFileConcurrent(t *testing.T) {

	dir := t.TempDir()
	want := make(map[string]struct{}, 10)
	for i := 0; i < 10; i++ {
		content := fmt.Sprintf("content-%d", i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		want[content] = struct{}{}
	}

	f := &file{
		Base: task.Base{
			Name:            "read",
			Type:            "file",
			TaskConcurrency: 4,
		},
		Path: config.String(filepath.Join(dir, "*.txt")),
	}

	out := make(chan *record.Record, 20)
	errCh := make(chan error, 1)
	go func() {
		errCh <- f.readFile(out)
		close(out)
	}()

	got := make(map[string]struct{})
	for r := range out {
		got[string(r.Data)] = struct{}{}
	}
	if err := <-errCh; err != nil {
		t.Fatalf("readFile: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for content := range want {
		if _, ok := got[content]; !ok {
			t.Errorf("missing content %q", content)
		}
	}

}
