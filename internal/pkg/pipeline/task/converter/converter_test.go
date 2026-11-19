package converter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/sstable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestConverterUnknownFormat(t *testing.T) {

	tk, err := New()
	require.NoError(t, err)
	assert.ErrorContains(t, yaml.Unmarshal([]byte("name: c\nformat: parquet"), tk), "invalid value for field format")

}

// The custom UnmarshalYAML copies only the name into task.Base, so type,
// fail_on_error, task_concurrency and context are silently dropped for every
// converter. Unskip once core.UnmarshalYAML decodes the embedded Base.
func TestConverterKeepsBaseFields(t *testing.T) {

	t.Skip("known bug: converter drops task.Base fields other than name (internal/pkg/pipeline/task/converter/converter.go:47-69)")

	tk := newTestTask(t, "name: c\ntype: converter\nformat: csv\nfail_on_error: true\ntask_concurrency: 2")
	assert.Equal(t, "converter", tk.GetType())
	assert.True(t, tk.GetFailOnError())
	assert.Equal(t, 2, tk.GetTaskConcurrency())

}

func TestCSVConverter(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		lines    []string
		expected []string
	}{
		{
			name: "Declared columns with numeric parsing",
			spec: `
skip_first: true
columns:
  - name: name
  - name: age
    is_numeric: true
  - name: salary
    is_numeric: true`,
			lines:    []string{"name,age,salary", "Ann,30,1000.5", "Bob,41,2000"},
			expected: []string{`{"age":30,"name":"Ann","salary":1000.5}`, `{"age":41,"name":"Bob","salary":2000}`},
		},
		{
			name:     "Header row becomes slugified column names",
			spec:     "skip_first: true",
			lines:    []string{"First Name,Last Name", "Ann,Lee"},
			expected: []string{`{"first_name":"Ann","last_name":"Lee"}`},
		},
		{
			name:     "No header and no columns auto-names them",
			spec:     "skip_first: false",
			lines:    []string{"Ann,Lee", "Bob,Ray"},
			expected: []string{`{"col1":"Ann","col2":"Lee"}`, `{"col1":"Bob","col2":"Ray"}`},
		},
		{
			name: "Declared columns without a header keep the first row",
			spec: `
columns:
  - name: a
  - name: b`,
			lines:    []string{"1,2", "3,4"},
			expected: []string{`{"a":"1","b":"2"}`, `{"a":"3","b":"4"}`},
		},
		{
			name:     "Quoted fields with embedded commas",
			spec:     "skip_first: true",
			lines:    []string{"id,note", `1,"hello, world"`},
			expected: []string{`{"id":"1","note":"hello, world"}`},
		},
		{
			name:     "UTF-8 BOM ahead of a quoted header is stripped",
			spec:     "skip_first: true",
			lines:    []string{"\xEF\xBB\xBF\"Order ID\",Total", "A1,9"},
			expected: []string{`{"order_id":"A1","total":"9"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloads := make([][]byte, len(tt.lines))
			for i, l := range tt.lines {
				payloads[i] = []byte(l)
			}
			emitted, err := runTestTask(t, newTestTask(t, "name: c\nformat: csv\n"+tt.spec), payloads...)
			require.NoError(t, err)
			require.Len(t, emitted, len(tt.expected))
			for i, want := range tt.expected {
				assert.JSONEq(t, want, string(emitted[i].Data))
			}
		})
	}

}

func TestCSVConverterErrors(t *testing.T) {

	numeric := "columns:\n  - name: n\n    is_numeric: true"

	tests := []struct {
		name  string
		spec  string
		lines []string
	}{
		{name: "Non-numeric value in a numeric column", spec: numeric, lines: []string{"abc"}},
		{name: "Empty numeric value", spec: numeric, lines: []string{" "}},
		{name: "Too few fields", spec: "columns:\n  - name: a\n  - name: b", lines: []string{"only"}},
		{name: "Too many fields", spec: "columns:\n  - name: a", lines: []string{"1,2"}},
		{name: "Malformed quoting", spec: "columns:\n  - name: a", lines: []string{`"unterminated`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloads := make([][]byte, len(tt.lines))
			for i, l := range tt.lines {
				payloads[i] = []byte(l)
			}
			_, err := runTestTask(t, newTestTask(t, "name: c\nformat: csv\n"+tt.spec), payloads...)
			assert.Error(t, err)
		})
	}

}

func TestToNumeric(t *testing.T) {

	tests := []struct {
		input string
		want  any
		ok    bool
	}{
		{input: "42", want: int64(42), ok: true},
		{input: " -7 ", want: int64(-7), ok: true},
		{input: "3.5", want: 3.5, ok: true},
		{input: "1e3", want: 1000.0, ok: true},
		{input: "", ok: false},
		{input: "1,000", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := toNumeric(tt.input)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}

}

func TestHTMLConverter(t *testing.T) {

	page := []byte(`<html><body><div class="card" id="c1">Hello <b>bold</b></div><div class="card">Two</div></body></html>`)

	t.Run("container selects nodes", func(t *testing.T) {
		emitted, err := runTestTask(t, newTestTask(t, "name: c\nformat: html\ncontainer: \"//div[@class='card']\""), page)
		require.NoError(t, err)
		require.Len(t, emitted, 1)
		assert.JSONEq(t, `[
			{"tag":"div","attributes":{"class":"card","id":"c1"},"text":"Hello","children":[{"tag":"b","text":"bold"}]},
			{"tag":"div","attributes":{"class":"card"},"text":"Two"}
		]`, string(emitted[0].Data))
	})

	t.Run("whole document without a container", func(t *testing.T) {
		emitted, err := runTestTask(t, newTestTask(t, "name: c\nformat: html"), []byte(`<p>x</p>`))
		require.NoError(t, err)
		require.Len(t, emitted, 1)

		var nodes []map[string]any
		require.NoError(t, json.Unmarshal(emitted[0].Data, &nodes))
		require.Len(t, nodes, 1)
		assert.Equal(t, "html", nodes[0]["children"].([]any)[0].(map[string]any)["tag"])
	})

	t.Run("missing container is an error", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: c\nformat: html\ncontainer: //table"), page)
		assert.ErrorContains(t, err, "no nodes found")
	})

}

func TestSSTConverter(t *testing.T) {

	t.Run("writes sorted keys readable by pebble", func(t *testing.T) {
		emitted, err := runTestTask(t, newTestTask(t, "name: c\nformat: sst\ndelimiter: '='"), []byte("b=2\na=1\n\nc=x=y\n"))
		require.NoError(t, err)
		require.Len(t, emitted, 1)

		assert.Equal(t, [][2]string{{"a", "1"}, {"b", "2"}, {"c", "x=y"}}, readSST(t, emitted[0].Data))
	})

	t.Run("line without delimiter is an error", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: c\nformat: sst\ndelimiter: '='"), []byte("novalue"))
		assert.ErrorContains(t, err, "expected 'key=value' format")
	})

}

func readSST(t *testing.T, data []byte) [][2]string {

	t.Helper()

	path := filepath.Join(t.TempDir(), "out.sst")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	f, err := os.Open(path)
	require.NoError(t, err)

	readable, err := sstable.NewSimpleReadable(f)
	require.NoError(t, err)

	reader, err := sstable.NewReader(readable, sstable.ReaderOptions{Comparer: pebble.DefaultComparer})
	require.NoError(t, err)
	defer reader.Close()

	iter, err := reader.NewIter(nil, nil)
	require.NoError(t, err)
	defer iter.Close()

	var pairs [][2]string
	for k, v := iter.First(); k != nil; k, v = iter.Next() {
		pairs = append(pairs, [2]string{string(k.UserKey), string(v.InPlaceValue())})
	}

	return pairs

}

func TestXLSXConverter(t *testing.T) {

	book := excelize.NewFile()
	require.NoError(t, book.SetSheetName("Sheet1", "Sales Q1"))
	require.NoError(t, book.SetSheetRow("Sales Q1", "A1", &[]any{"title row"}))
	require.NoError(t, book.SetSheetRow("Sales Q1", "A2", &[]any{"Item Name", "Unit Price"}))
	require.NoError(t, book.SetSheetRow("Sales Q1", "A3", &[]any{"pen", 2}))
	_, err := book.NewSheet("Other")
	require.NoError(t, err)
	require.NoError(t, book.SetSheetRow("Other", "A1", &[]any{"k", "v"}))
	buf, err := book.WriteToBuffer()
	require.NoError(t, err)
	workbook := buf.Bytes()

	t.Run("one record per sheet with sheet name in context", func(t *testing.T) {
		emitted, err := runTestTask(t, newTestTask(t, "name: c\nformat: xlsx"), workbook)
		require.NoError(t, err)
		require.Len(t, emitted, 2)

		assert.Equal(t, "title row\nItem Name,Unit Price\npen,2\n", string(emitted[0].Data))
		name, ok := emitted[0].GetContextValue(sheetName)
		assert.True(t, ok)
		assert.Equal(t, "Sales Q1", name)

		assert.Equal(t, "k,v\n", string(emitted[1].Data))
	})

	t.Run("skip rows, sanitise headers and sheet names", func(t *testing.T) {
		spec := `
name: c
format: xlsx
sheets: ["Sales Q1"]
skip_rows_by_sheet:
  Sales Q1: 1
sanitize_headers: true
sanitize_sheet_names: true`
		emitted, err := runTestTask(t, newTestTask(t, spec), workbook)
		require.NoError(t, err)
		require.Len(t, emitted, 1)

		assert.Equal(t, "item_name,unit_price\npen,2\n", string(emitted[0].Data))
		name, _ := emitted[0].GetContextValue(sheetName)
		assert.Equal(t, "sales_q1", name)
	})

	t.Run("negative skip is treated as zero", func(t *testing.T) {
		x := &xlsx{SkipRows: -2}
		assert.Equal(t, 0, x.getRowsToSkip("any"))
	})

	t.Run("unknown sheet is an error", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: c\nformat: xlsx\nsheets: [Missing]"), workbook)
		assert.Error(t, err)
	})

	t.Run("not a workbook", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: c\nformat: xlsx"), []byte("plain text"))
		assert.Error(t, err)
	})

}

func TestEMLConverter(t *testing.T) {

	mail, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "test", "pipelines", "converter", "test_mail"))
	require.NoError(t, err)

	emitted, err := runTestTask(t, newTestTask(t, "name: c\nformat: eml"), mail)
	require.NoError(t, err)

	byName := map[string]*record.Record{}
	for _, r := range emitted {
		name, ok := r.GetContextValue("converter_filename")
		require.True(t, ok)
		byName[name] = r
	}

	require.Contains(t, byName, "body.txt")
	assert.Contains(t, string(byName["body.txt"].Data), "Please find attached")
	contentType, _ := byName["body.txt"].GetContextValue("content_type")
	assert.Equal(t, "text/plain", contentType)

	require.Contains(t, byName, "headers.json")
	var headers map[string]string
	require.NoError(t, json.Unmarshal(byName["headers.json"].Data, &headers))
	assert.Equal(t, "sender@example.com", headers["From"])

	require.Contains(t, byName, "test.pdf")
	assert.Equal(t, "%PDF", string(byName["test.pdf"].Data[:4]))
	contentType, _ = byName["test.pdf"].GetContextValue("content_type")
	assert.Equal(t, "application/pdf", contentType)

}

func TestEMLProcessOutput(t *testing.T) {

	c := &eml{}

	t.Run("empty content is dropped", func(t *testing.T) {
		assert.Nil(t, c.processOutput(nil, "x.txt", "text/plain"))
	})

	t.Run("path components are stripped and the name slugified", func(t *testing.T) {
		out := c.processOutput([]byte("x"), "../../etc/My Report.CSV", "text/csv")
		require.NotNil(t, out)
		assert.Equal(t, "my_report.csv", out.Metadata["converter_filename"])
	})

	t.Run("content type falls back to the extension", func(t *testing.T) {
		out := c.processOutput([]byte("x"), "page.html", "")
		require.NotNil(t, out)
		assert.Contains(t, out.Metadata["content_type"], "text/html")
	})

	t.Run("unknown extension falls back to octet-stream", func(t *testing.T) {
		out := c.processOutput([]byte("x"), "blob.zzz-unknown", "")
		require.NotNil(t, out)
		assert.Equal(t, "application/octet-stream", out.Metadata["content_type"])
	})

}

func TestConverterIgnoresNilInput(t *testing.T) {

	tk := newTestTask(t, "name: c\nformat: csv")
	assert.NoError(t, tk.Run(nil, nil))

}

// newTestTask builds the task from YAML exactly as a pipeline would.
func newTestTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := New()
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal([]byte(spec), tk))
	require.NoError(t, tk.Init())

	return tk

}

// runTestTask feeds one record per payload through Run and collects everything emitted.
func runTestTask(t *testing.T, tk task.Task, payloads ...[]byte) ([]*record.Record, error) {

	t.Helper()

	in := make(chan *record.Record, len(payloads))
	for i, p := range payloads {
		in <- &record.Record{ID: i + 1, Data: p, Context: context.Background()}
	}
	close(in)

	out := make(chan *record.Record, 4096)
	err := tk.Run(in, out)
	close(out)

	var emitted []*record.Record
	for r := range out {
		emitted = append(emitted, r)
	}

	return emitted, err

}
