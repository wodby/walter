package walter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wodby/walter/config"
)

func TestLargeOutputDoesNotBlockCommands(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WALTER_OUTPUT_TEST_DIR", dir)
	path := filepath.Join(dir, "pipeline.yml")
	data := `pipeline:
  - name: output
    command: yes x | head -c 150000
  - name: next
    command: touch "$WALTER_OUTPUT_TEST_DIR/next"
  - name: read_file
    command: test "$(wc -c < "$__OUT_FILE__output__")" -eq 150000
  - name: small
    command: printf small_value
  - name: consume
    command: test "$__OUT__small__" = small_value
    only_if: test "$__RESULT__small__" = true
cleanup:
  - name: cleanup
    command: test "$(wc -c < "$__OUT_FILE__output__")" -eq 150000
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	runner, err := New(&config.Opts{PipelineFilePath: path, Mode: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if !runner.Run() {
		t.Fatal("large output broke pipeline")
	}
	if _, err := os.Stat(filepath.Join(dir, "next")); err != nil {
		t.Fatal("next command did not execute")
	}
}
