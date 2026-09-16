package stages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShellScriptFilenameIsLiteral(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "script ' ; false; #.sh")
	if err := os.WriteFile(file, []byte("printf literal"), 0600); err != nil {
		t.Fatal(err)
	}
	stage := NewShellScriptStage()
	stage.File = file
	if !stage.Run() || stage.OutResult != "literal" {
		t.Fatalf("literal filename failed: %q", stage.OutResult)
	}
}

func TestCommandFailuresDoNotPanic(t *testing.T) {
	stage := &CommandStage{Command: "true", WaitFor: "invalid"}
	if stage.Run() {
		t.Fatal("invalid wait accepted")
	}
	stage.WaitFor = ""
	stage.Directory = filepath.Join(t.TempDir(), "missing")
	if stage.Run() {
		t.Fatal("invalid working directory accepted")
	}
}
