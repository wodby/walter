package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRunLockHelper(t *testing.T) {
	path := os.Getenv("WALTER_LOCK_HELPER")
	if path == "" {
		return
	}
	if _, err := AcquireRunLock(path); err != nil {
		os.Exit(2)
	}
	// Deliberately exit without calling release, as after an abrupt process exit.
	os.Exit(0)
}

func TestRunLockAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	release, err := AcquireRunLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := func() error {
		c := exec.Command(binary, "-test.run=^TestRunLockHelper$")
		c.Env = append(os.Environ(), "WALTER_LOCK_HELPER="+path)
		return c.Run()
	}
	if err := child(); err == nil {
		t.Fatal("second process acquired held lock")
	}
	release()
	if err := child(); err != nil {
		t.Fatalf("released lock unavailable: %v", err)
	}
	again, err := AcquireRunLock(path)
	if err != nil {
		t.Fatalf("process exit retained lock: %v", err)
	}
	again()
	if _, err := os.Stat(path + ".lock"); err != nil {
		t.Fatal("lock sidecar was removed")
	}
}

func TestRunLockRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	path := filepath.Join(dir, "state")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path+".lock"); err != nil {
		t.Fatal(err)
	}
	if release, err := AcquireRunLock(path); err == nil {
		release()
		t.Fatal("followed lock symlink")
	}
}
