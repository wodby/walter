package config

import (
	"os"
	"strings"
	"testing"
)

func TestCommandEnvironmentBoundsAndFiles(t *testing.T) {
	e := NewEnvVariables()
	defer e.Close()
	large := strings.Repeat("x", 150000)
	e.ExportSpecialVariable(`__OUT["large"]`, large)
	if _, ok := os.LookupEnv("__OUT__large__"); ok {
		t.Fatal("result leaked into global environment")
	}
	env, err := e.CommandEnvironment("true")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(env, "\n"), "__OUT__large__=") {
		t.Fatal("unrequested output inherited")
	}
	if _, err = e.CommandEnvironment("echo $__OUT__large__"); err == nil {
		t.Fatal("oversized requested output accepted")
	}
	env, err = e.CommandEnvironment(`cat "${__OUT_FILE__large__}"`)
	if err != nil {
		t.Fatal(err)
	}
	var path string
	for _, entry := range env {
		if strings.HasPrefix(entry, "__OUT_FILE__large__=") {
			path = strings.TrimPrefix(entry, "__OUT_FILE__large__=")
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != large {
		t.Fatalf("result file mismatch: %v", err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("result file is not private")
	}
	e.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("result file leaked after cleanup")
	}
}

func TestRequestedResultEnvironmentLimit(t *testing.T) {
	e := NewEnvVariables()
	defer e.Close()
	for _, key := range []string{"a", "b", "c"} {
		e.ExportSpecialVariable("__OUT__"+key+"__", strings.Repeat("x", 30<<10))
	}
	if _, err := e.CommandEnvironment("echo $__OUT__a__ $__OUT__b__ $__OUT__c__"); err == nil {
		t.Fatal("aggregate limit not enforced")
	}
	e.ExportSpecialVariable("__OUT__nul__", "a\x00b")
	if _, err := e.CommandEnvironment("echo $__OUT__nul__"); err == nil {
		t.Fatal("NUL accepted into environment")
	}
	if _, err := e.CommandEnvironment("cat $__OUT_FILE__nul__"); err != nil {
		t.Fatal(err)
	}
}
