package config

import "testing"

func TestEnvironmentPreservesEquals(t *testing.T) {
	t.Setenv("WALTER_TEST_SECRET", "value=with==padding")
	got, ok := NewEnvVariables().Get("WALTER_TEST_SECRET")
	if !ok || got != "value=with==padding" {
		t.Fatalf("truncated environment: %q", got)
	}
}
