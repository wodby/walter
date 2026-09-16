package walter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wodby/walter/config"
)

func TestParallelStageResults(t *testing.T) {
	var data strings.Builder
	data.WriteString("pipeline:\n  - name: parent\n    command: 'true'\n    parallel:\n")
	for i := 0; i < 32; i++ {
		fmt.Fprintf(&data, "      - name: worker_%d\n        command: printf result_%d\n", i, i)
	}
	path := filepath.Join(t.TempDir(), "pipeline.yml")
	if err := os.WriteFile(path, []byte(data.String()), 0600); err != nil {
		t.Fatal(err)
	}
	runner, err := New(&config.Opts{PipelineFilePath: path, Mode: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if !runner.Run() {
		t.Fatal("parallel pipeline failed")
	}
	for i := 0; i < 32; i++ {
		got, ok := runner.Engine.EnvVariables.Get(fmt.Sprintf("__OUT[\"worker_%d\"]", i))
		if !ok || got != fmt.Sprintf("result_%d", i) {
			t.Fatalf("worker %d lost output: %q", i, got)
		}
	}
}
