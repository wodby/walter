package config

import (
	"fmt"
	"sync"
	"testing"
)

func TestConcurrentResultAccess(t *testing.T) {
	env := NewEnvVariables()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("__OUT[\"race_%d\"]", i)
			for n := 0; n < 100; n++ {
				env.ExportSpecialVariable(key, "result")
				if got, ok := env.Get(key); !ok || got != "result" {
					t.Errorf("lost result for %s", key)
				}
				env.Add("SHARED", "value")
				env.Replace("$SHARED")
			}
		}(i)
	}
	wg.Wait()
}
