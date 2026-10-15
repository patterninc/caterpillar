package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"gopkg.in/yaml.v3"
)

// BenchmarkConcurrentReadContext times a reader with eight fromjson context
// fields, the shape that used to queue every worker on one task lock.
// 64 local JSON files, task_concurrency 10.
func BenchmarkConcurrentReadContext(b *testing.B) {

	dir := b.TempDir()
	const files = 64
	pad := strings.Repeat("x", 4096)
	for i := range files {
		body := fmt.Sprintf(
			`{"id":%d,"name":"user-%d","email":"user-%d@example.com","company":{"name":"Acme"},"address":{"city":"Pune"},"active":true,"tags":["a","b"],"score":%d,"pad":"%s"}`,
			i, i, i, i, pad,
		)
		path := filepath.Join(dir, fmt.Sprintf("file-%02d.json", i))
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			b.Fatal(err)
		}
	}

	spec := `
name: read_bench
type: file
path: ` + filepath.Join(dir, "*.json") + `
task_concurrency: 10
fail_on_error: true
context:
  user_id: '.data | fromjson | .id'
  user_name: '.data | fromjson | .name'
  user_email: '.data | fromjson | .email'
  user_company: '.data | fromjson | .company.name'
  city: '.data | fromjson | .address.city'
  active: '.data | fromjson | .active'
  tag_count: '.data | fromjson | .tags | length'
  score: '.data | fromjson | .score'
`

	b.ResetTimer()
	for range b.N {
		tk, err := New()
		if err != nil {
			b.Fatal(err)
		}
		if err := yaml.Unmarshal([]byte(spec), tk); err != nil {
			b.Fatal(err)
		}
		out := make(chan *record.Record, files)
		if err := tk.Run(nil, out); err != nil {
			b.Fatal(err)
		}
		if got := len(out); got != files {
			b.Fatalf("emitted %d records, want %d", got, files)
		}
	}

}
