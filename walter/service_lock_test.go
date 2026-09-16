package walter

import (
	"container/list"
	"os"
	"path/filepath"
	"testing"

	"github.com/wodby/walter/engine"
	"github.com/wodby/walter/pipelines"
	"github.com/wodby/walter/services"
)

type lockTestService struct {
	path   string
	called bool
}

func (s *lockTestService) GetUpdateFilePath() string            { return s.path }
func (s *lockTestService) RegisterResult(services.Result) error { return nil }
func (s *lockTestService) GetCommits(services.Update) (*list.List, error) {
	s.called = true
	return list.New(), nil
}

func TestServiceStateErrorsPreventExecution(t *testing.T) {
	for _, state := range []string{`{"status":"inprogress"}`, `broken json`} {
		path := filepath.Join(t.TempDir(), "state")
		if err := os.WriteFile(path, []byte(state), 0600); err != nil {
			t.Fatal(err)
		}
		s := &lockTestService{path: path}
		w := &Walter{Engine: &engine.Engine{Resources: &pipelines.Resources{RepoService: s}}}
		if w.runService() || s.called {
			t.Fatal("invalid or active state allowed execution")
		}
		data, _ := os.ReadFile(path)
		if string(data) != state {
			t.Fatal("state overwritten after load error")
		}
		release, err := services.AcquireRunLock(path)
		if err != nil {
			t.Fatal("error path retained lock")
		}
		release()
	}
}
