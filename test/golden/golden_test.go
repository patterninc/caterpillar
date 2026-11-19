// Package golden runs selected example pipelines end to end and compares what
// they print with committed golden files. Regenerate with:
//
//	go test ./test/golden/... -run Golden -update
package golden

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline"
)

var update = flag.Bool(`update`, false, `rewrite golden files with the current output`)

// Only pipelines that are offline and whose stdout order is fixed belong here: a
// linear chain with one worker per task keeps record order, while DAG fan-out,
// sampling at random and shuffle do not.
var goldenPipelines = []string{
	`bcrypt_test`,
	`empty`,
	`file`,
	`hash_test`,
	`join`,
	`jq_error_not_ignored_test`,
	`jq_error_skipped_test`,
	`jq_error_test`,
	`unknown`,
	`xpath`,
	`xpath_with_index`,
}

// echo stamps every line with the wall clock, which says nothing about the pipeline.
var echoTimestamp = regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} - `)

// Diagnostics come from whichever task hit them, so their position relative to
// echoed records depends on scheduling. They are compared as a sorted set instead.
var diagnosticPrefixes = []string{`error in `, `error finishing `, `WARN: `}

func TestGolden(t *testing.T) {

	golden, err := filepath.Abs(`testdata`)
	require.NoError(t, err)

	// pipeline paths such as test/pipelines/names.txt are relative to the repo root
	t.Chdir(filepath.Join(`..`, `..`))

	for _, name := range goldenPipelines {
		t.Run(name, func(t *testing.T) {
			got := runPipeline(t, filepath.Join(`test`, `pipelines`, name+`.yaml`))
			path := filepath.Join(golden, name+`.golden`)

			if *update {
				require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
				return
			}

			want, err := os.ReadFile(path)
			require.NoError(t, err, "missing golden file: run go test ./test/golden/... -run Golden -update")
			require.Equal(t, string(want), got)
		})
	}

}

// runPipeline does what cmd/caterpillar does, in process, and returns the
// normalised stdout followed by the outcome.
func runPipeline(t *testing.T, conf string) string {

	t.Helper()

	var runErr error
	stdout := captureStdout(t, func() {
		p := &pipeline.Pipeline{}
		if runErr = config.Load(conf, p); runErr != nil {
			return
		}
		runErr = p.Run()
	})

	var records, diagnostics []string
	for _, line := range strings.Split(echoTimestamp.ReplaceAllString(stdout, ``), "\n") {
		if isDiagnostic(line) {
			diagnostics = append(diagnostics, line)
			continue
		}
		records = append(records, line)
	}
	sort.Strings(diagnostics)

	outcome := `ok`
	if runErr != nil {
		outcome = `error: ` + runErr.Error()
	}

	var out strings.Builder
	out.WriteString("# stdout\n")
	out.WriteString(strings.TrimRight(strings.Join(records, "\n"), "\n"))
	out.WriteString("\n# diagnostics\n")
	for _, d := range diagnostics {
		out.WriteString(d + "\n")
	}
	out.WriteString("# result\n")
	out.WriteString(strings.TrimRight(outcome, "\n") + "\n")

	return out.String()

}

func isDiagnostic(line string) bool {

	for _, p := range diagnosticPrefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}

	return false

}

func captureStdout(t *testing.T, fn func()) string {

	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	require.NoError(t, w.Close())

	return <-done

}
