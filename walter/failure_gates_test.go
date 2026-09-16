package walter

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/wodby/walter/config"
)

// Markers verify real command side effects and cleanup, not just result labels.
func TestFailureGates(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(fmt.Sprintf("stop_%v", stop), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("WALTER_GATE_TEST_DIR", dir)
			path := filepath.Join(dir, "pipeline.yml")
			data := `pipeline:
  - name: gate
    command: exit 1
    parallel:
      - name: dependent
        command: touch "$WALTER_GATE_TEST_DIR/dependent"
  - name: independent
    command: touch "$WALTER_GATE_TEST_DIR/independent"
cleanup:
  - name: cleanup
    command: touch "$WALTER_GATE_TEST_DIR/cleanup"
`
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			runner, err := New(&config.Opts{PipelineFilePath: path, Mode: "local", StopOnAnyFailure: stop})
			if err != nil {
				t.Fatal(err)
			}
			if runner.Run() {
				t.Fatal("failed gate reported success")
			}
			for name, want := range map[string]bool{"dependent": false, "independent": !stop, "cleanup": true} {
				_, err := os.Stat(filepath.Join(dir, name))
				got := err == nil
				if got != want {
					t.Errorf("%s executed=%v, want %v", name, got, want)
				}
			}
		})
	}
}
