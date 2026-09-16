package walter

import (
	"container/list"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v92/github"
	"github.com/wodby/walter/config"
	"github.com/wodby/walter/engine"
	"github.com/wodby/walter/pipelines"
	"github.com/wodby/walter/services"
)

type commitTestService struct {
	results  []services.Result
	path     string
	commits  *list.List
	observed time.Time
	err      error
}

func (s *commitTestService) GetUpdateFilePath() string { return s.path }
func (s *commitTestService) GetCommits(services.Update) (*list.List, error) {
	s.observed = time.Now()
	if s.commits != nil {
		return s.commits, nil
	}
	return list.New(), nil
}
func (s *commitTestService) RegisterResult(r services.Result) error {
	s.results = append(s.results, r)
	return s.err
}

// Reuse existing commit objects; the test creates no commits and uses local Git only.
func commitFixture(t *testing.T) (string, string) {
	t.Helper()
	source, err := gitOutput(".", "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	sha, err := gitOutput(source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "repo")
	if output, err := exec.Command("git", "clone", "--no-hardlinks", source, root).CombinedOutput(); err != nil {
		t.Fatalf("clone: %s: %v", output, err)
	}
	t.Chdir(root)
	return root, sha
}

func TestExactCommitIsolationAndReporting(t *testing.T) {
	root, sha := commitFixture(t)
	if err := os.WriteFile(filepath.Join(root, "untracked"), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	original, _ := gitOutput(root, "rev-parse", "HEAD")
	for _, tc := range []struct {
		name        string
		passed      bool
		reportError error
	}{
		{"success", true, nil}, {"failure", false, nil}, {"report_error", true, errors.New("unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &commitTestService{err: tc.reportError}
			runner := &Walter{Opts: &config.Opts{PipelineFilePath: "README.md"}, Engine: &engine.Engine{Resources: &pipelines.Resources{RepoService: service}}}
			var testedPath string
			got := runner.processCommit(sha, "", func(dir, pipeline string) bool {
				testedPath = dir
				head, err := gitOutput(dir, "rev-parse", "HEAD")
				if err != nil || head != sha {
					t.Fatal("wrong SHA tested")
				}
				if dir == root || pipeline != "README.md" {
					t.Fatal("not isolated or wrong pipeline path")
				}
				return tc.passed
			})
			if got != (tc.passed && tc.reportError == nil) {
				t.Fatal("incorrect returned result")
			}
			if len(service.results) != 1 || service.results[0].SHA != sha {
				t.Fatal("wrong reported SHA")
			}
			if (service.results[0].State == "success") != tc.passed {
				t.Fatal("wrong reported status")
			}
			if _, err := os.Stat(testedPath); !os.IsNotExist(err) {
				t.Fatal("worktree not removed")
			}
		})
	}
	head, _ := gitOutput(root, "rev-parse", "HEAD")
	if head != original {
		t.Fatal("original checkout changed")
	}
	if b, err := os.ReadFile(filepath.Join(root, "untracked")); err != nil || string(b) != "preserve" {
		t.Fatal("user file changed")
	}
}

func TestChangedRemoteRevisionIsNotReported(t *testing.T) {
	_, sha := commitFixture(t)
	service := &commitTestService{}
	runner := &Walter{Opts: &config.Opts{PipelineFilePath: "README.md"}, Engine: &engine.Engine{Resources: &pipelines.Resources{RepoService: service}}}
	wrong := strings.Repeat("1", 40)
	if wrong == sha {
		wrong = strings.Repeat("2", 40)
	}
	if runner.processCommit(wrong, "HEAD", func(string, string) bool { t.Fatal("ran wrong revision"); return true }) {
		t.Fatal("mismatch accepted")
	}
	if len(service.results) != 0 {
		t.Fatal("reported an untested revision")
	}
	if runner.processCommit("--upload-pack=bad", "", func(string, string) bool { return true }) {
		t.Fatal("invalid SHA accepted")
	}
}

func TestChildPipelineWorkingDirectory(t *testing.T) {
	source, err := gitOutput(".", "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "walter")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = source
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %s: %v", output, err)
	}
	checkout := t.TempDir()
	t.Setenv("WALTER_EXPECTED_DIR", checkout)
	for name, data := range map[string]string{
		"pipeline.yml": "pipeline:\n  - name: paths\n    command: test \"$PWD\" = \"$WALTER_EXPECTED_DIR\" && sh script.sh\n",
		"script.sh":    "test -f marker\n", "marker": "present",
		"failure.yml": "pipeline:\n  - name: failed\n    command: exit 1\n",
	} {
		if err := os.WriteFile(filepath.Join(checkout, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	runner := &Walter{Opts: &config.Opts{}}
	if !runner.runPipelineProcess(binary, checkout, "pipeline.yml") {
		t.Fatal("child used incorrect paths")
	}
	if runner.runPipelineProcess(binary, checkout, "failure.yml") {
		t.Fatal("failed child reported success")
	}
}

func TestDiscoveryCheckpointRetainedOnFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "state")
		previous := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		if !services.SaveLastUpdate(path, services.Update{Time: previous, Status: "finished", Succeeded: true}) {
			t.Fatal("save failed")
		}
		service := &commitTestService{path: path, commits: list.New()}
		if fail {
			service.commits.PushBack(github.RepositoryCommit{})
		}
		runner := &Walter{Opts: &config.Opts{}, Engine: &engine.Engine{Resources: &pipelines.Resources{RepoService: service}}}
		if runner.runService() == fail {
			t.Fatal("wrong service result")
		}
		state, err := services.LoadLastUpdate(path)
		if err != nil {
			t.Fatal(err)
		}
		if fail && (!state.Time.Equal(previous) || state.Succeeded) {
			t.Fatal("failed revision advanced checkpoint")
		}
		if !fail && (!state.Time.After(previous) || state.Time.After(service.observed)) {
			t.Fatal("checkpoint did not use discovery start")
		}
	}
}

func TestPipelineChangingHEADIsNotReported(t *testing.T) {
	_, sha := commitFixture(t)
	service := &commitTestService{}
	runner := &Walter{Opts: &config.Opts{PipelineFilePath: "README.md"}, Engine: &engine.Engine{Resources: &pipelines.Resources{RepoService: service}}}
	if runner.processCommit(sha, "", func(dir, stringUnused string) bool {
		if _, err := gitOutput(dir, "symbolic-ref", "HEAD", "refs/heads/unborn-test"); err != nil {
			t.Fatal(err)
		}
		return true
	}) {
		t.Fatal("changed HEAD accepted")
	}
	if len(service.results) != 0 {
		t.Fatal("reported success after HEAD changed")
	}
}
