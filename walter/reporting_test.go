package walter

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wodby/walter/config"
)

// Exercise the parser and actual HTTP reporter together, using only a local receiver.
func TestOutputReportingBoolean(t *testing.T) {
	for _, tc := range []struct {
		name, setting         string
		wantOutput, wantError bool
	}{
		{"omitted", "", false, false}, {"false", "    report_full_output: false\n", false, false},
		{"true", "    report_full_output: true\n", true, false},
		{"string", "    report_full_output: 'false'\n", false, true},
		{"null", "    report_full_output: null\n", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var messages []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				mu.Lock()
				messages = append(messages, r.Form.Get("payload"))
				mu.Unlock()
				w.Write([]byte("ok"))
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "pipeline.yml")
			data := "messenger:\n  type: slack\n  channel: test\n  url: " + server.URL + "\npipeline:\n  - name: output\n    command: echo TEST_OUTPUT_SENTINEL\n" + tc.setting
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			runner, err := New(&config.Opts{PipelineFilePath: path, Mode: "local"})
			if tc.wantError {
				if err == nil {
					t.Fatal("invalid boolean accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !runner.Run() {
				t.Fatal("pipeline failed")
			}
			mu.Lock()
			defer mu.Unlock()
			if len(messages) == 0 {
				t.Fatal("expected result notification")
			}
			got := strings.Contains(strings.Join(messages, "\n"), "TEST_OUTPUT_SENTINEL")
			if got != tc.wantOutput {
				t.Fatalf("output sent: %v, want %v", got, tc.wantOutput)
			}
		})
	}
}
