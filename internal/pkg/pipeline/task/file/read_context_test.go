package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"gopkg.in/yaml.v3"
)

func TestReadEvaluatesContextConcurrently(t *testing.T) {

	dir := t.TempDir()
	for _, item := range []struct {
		name string
		body string
	}{
		{"a.json", `{"id":1,"name":"Ada","email":"ada@example.com","company":{"name":"Analytical"},"address":{"city":"London"},"active":true,"tags":["math"],"score":1}`},
		{"b.json", `{"id":2,"name":"Grace","email":"grace@example.com","company":{"name":"Navy"},"address":{"city":"New York"},"active":false,"tags":["cobol","compilers"],"score":2}`},
		{"nested/c.json", `{"id":3,"name":"Lin","email":"lin@example.com","company":{"name":"Bell"},"address":{"city":"Murray Hill"},"active":true,"tags":["unix","c","go"],"score":3}`},
		{"nested/d.json", `{"id":4,"name":"Ken","email":"ken@example.com","company":{"name":"Bell"},"address":{"city":"Murray Hill"},"active":true,"tags":["unix"],"score":4}`},
	} {
		path := filepath.Join(dir, item.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(item.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tk, err := New()
	if err != nil {
		t.Fatal(err)
	}
	spec := `
name: read_nested_concurrently
type: file
path: ` + filepath.Join(dir, "**/*.json") + `
task_concurrency: 4
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
	if err := yaml.Unmarshal([]byte(spec), tk); err != nil {
		t.Fatal(err)
	}

	out := make(chan *record.Record, 4)
	if err := tk.Run(nil, out); err != nil {
		t.Fatal(err)
	}

	seen := map[string]string{}
	for range 4 {
		got := <-out

		name, ok := got.GetContextValue(string(task.CtxKeyFileNameWrite))
		if !ok || name == "" {
			t.Fatalf("missing file name on record %d", got.ID)
		}
		if _, dup := seen[name]; dup {
			t.Fatalf("duplicate file %s", name)
		}
		seen[name] = string(got.Data)

		var body map[string]any
		if err := json.Unmarshal(got.Data, &body); err != nil {
			t.Fatal(err)
		}
		assertContext := func(key, want string) {
			t.Helper()
			gotValue, ok := got.GetContextValue(key)
			if !ok || gotValue != want {
				t.Errorf("%s %s: got %q, want %q", name, key, gotValue, want)
			}
		}
		idNumber, _ := body["id"].(float64)
		assertContext("user_id", jsonNumber(idNumber))
		assertContext("user_name", jsonString(body["name"]))
		assertContext("user_email", jsonString(body["email"]))
		company, _ := body["company"].(map[string]any)
		assertContext("user_company", jsonString(company["name"]))
		address, _ := body["address"].(map[string]any)
		assertContext("city", jsonString(address["city"]))
		assertContext("active", jsonBool(body["active"]))
		tags, _ := body["tags"].([]any)
		assertContext("tag_count", jsonNumber(float64(len(tags))))
		score, _ := body["score"].(float64)
		assertContext("score", jsonNumber(score))
	}

	if len(seen) != 4 {
		t.Fatalf("read %d files, want 4: %v", len(seen), seen)
	}

}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func jsonNumber(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func jsonBool(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
