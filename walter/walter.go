/* walter: a deployment pipeline template
* Copyright (C) 2014 Recruit Technologies Co., Ltd. and contributors
* (see CONTRIBUTORS.md)
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
* http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

// Package walter is the main package for the walter application
package walter

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v92/github"
	"github.com/wodby/walter/config"
	"github.com/wodby/walter/engine"
	"github.com/wodby/walter/log"
	"github.com/wodby/walter/services"
	"github.com/wodby/walter/stages"
)

// Walter object.
type Walter struct {
	Engine *engine.Engine
	Opts   *config.Opts
}

// New creates a Walter instance.
func New(opts *config.Opts) (*Walter, error) {
	log.Infof("Pipeline file path: \"%s\"", opts.PipelineFilePath)
	configData, err := config.ReadConfig(opts.PipelineFilePath)
	if err != nil {
		log.Warn("failed to read the configuration file")
		return nil, err
	}

	envs := config.NewEnvVariables()
	parser := &config.Parser{ConfigData: configData, EnvVariables: envs}
	resources, err := parser.Parse()
	if err != nil {
		log.Warn("failed to parse the configuration")
		return nil, err
	}
	monitorCh := make(chan stages.Mediator)
	engine := &engine.Engine{
		Resources:    resources,
		Opts:         opts,
		MonitorCh:    &monitorCh,
		EnvVariables: envs,
	}
	return &Walter{
		Opts:   opts,
		Engine: engine,
	}, nil
}

// Run executes registered Pipeline.
func (e *Walter) Run() bool {
	repoServiceValue := reflect.ValueOf(e.Engine.Resources.RepoService)
	if e.Engine.Opts.Mode == "local" ||
		repoServiceValue.Type().String() == "*services.LocalClient" {
		log.Info("Starting Walter in local mode")
		result := e.Engine.RunOnce()
		return result.IsSucceeded()
	}
	log.Info("Starting Walter in repository service mode")
	return e.runService()

}

func (e *Walter) runService() bool {
	release, err := services.AcquireRunLock(e.Engine.Resources.RepoService.GetUpdateFilePath())
	if err != nil {
		log.Errorf("Cannot start service run: %s", err)
		return false
	}
	defer release()
	// load .walter-update
	log.Infof("Loading update file... \"%s\"", e.Engine.Resources.RepoService.GetUpdateFilePath())
	update, err := services.LoadLastUpdate(e.Engine.Resources.RepoService.GetUpdateFilePath())
	if err != nil {
		log.Errorf("Cannot load service state: %s", err)
		return false
	}
	log.Infof("Succeeded loading update file")

	log.Info("Updating status...")
	update.Status = "inprogress"
	result := services.SaveLastUpdate(e.Engine.Resources.RepoService.GetUpdateFilePath(), update)
	if result == false {
		log.Error("Failed to save status update")
		return false
	}
	log.Info("Succeeded updating status")

	// get latest commit and pull requests
	log.Info("downloading commits and pull requests...")
	polledAt := time.Now()
	commits, err := e.Engine.Resources.RepoService.GetCommits(update)
	if err != nil {
		log.Errorf("Failed getting commits: %s", err)
		return false
	}

	log.Info("Succeeded getting commits")
	log.Info("Size of commits: " + strconv.Itoa(commits.Len()))
	hasFailedProcess := false
	for commit := commits.Front(); commit != nil; commit = commit.Next() {
		commitType := reflect.TypeOf(commit.Value)
		if commitType.Name() == "RepositoryCommit" {
			log.Info("Found new repository commit")
			trunkCommit := commit.Value.(github.RepositoryCommit)
			if result := e.processTrunkCommit(trunkCommit); result == false {
				hasFailedProcess = true
			}
		} else if commitType.Name() == "PullRequest" {
			log.Info("Found new pull request commit")
			pullreq := commit.Value.(github.PullRequest)
			if result := e.processPullRequest(pullreq); result == false {
				hasFailedProcess = true
			}
		} else {
			log.Errorf("Nothing commit type: %s", commitType)
			hasFailedProcess = true
		}
	}

	// save .walter-update
	log.Info("Saving update file...")
	update.Status = "finished"
	if !hasFailedProcess {
		update.Time = polledAt
	}
	update.Succeeded = !hasFailedProcess
	result = services.SaveLastUpdate(e.Engine.Resources.RepoService.GetUpdateFilePath(), update)
	if result == false {
		log.Error("Failed to save update")
		return false
	}
	return !hasFailedProcess
}

func (e *Walter) processTrunkCommit(commit github.RepositoryCommit) bool {
	return e.processCommit(commit.GetSHA(), "", e.runWorktreePipeline)
}

func (e *Walter) processPullRequest(pullrequest github.PullRequest) bool {
	if pullrequest.GetNumber() <= 0 {
		log.Error("Invalid pull request number")
		return false
	}
	return e.processCommit(pullrequest.GetHead().GetSHA(), "refs/pull/"+strconv.Itoa(pullrequest.GetNumber())+"/head", e.runWorktreePipeline)
}

var commitID = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// gitOutput runs git without shell interpolation and keeps operations in one repository.
func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}

// processCommit runs an immutable revision in an isolated worktree and reports
// success only while HEAD still identifies that exact revision.
func (e *Walter) processCommit(sha, ref string, run func(string, string) bool) bool {
	if !commitID.MatchString(sha) {
		log.Error("Invalid commit SHA")
		return false
	}
	sha = strings.ToLower(sha)
	root, err := gitOutput(".", "rev-parse", "--show-toplevel")
	if err != nil {
		log.Error("Cannot locate repository")
		return false
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	pipeline, err := filepath.Abs(e.Opts.PipelineFilePath)
	if err != nil {
		return false
	}
	pipeline, err = filepath.EvalSymlinks(pipeline)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, pipeline)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		log.Error("Service pipeline must be inside the repository")
		return false
	}
	if ref == "" {
		ref = sha
	}
	if _, err = gitOutput(root, "fetch", "--no-tags", "origin", ref); err != nil {
		log.Error("Cannot fetch requested revision")
		return false
	}
	fetched, err := gitOutput(root, "rev-parse", "FETCH_HEAD^{commit}")
	if err != nil || fetched != sha {
		log.Error("Remote revision changed; refusing to test a different commit")
		return false
	}
	temporary, err := os.MkdirTemp("", "walter-checkout-*")
	if err != nil {
		return false
	}
	defer os.RemoveAll(temporary)
	checkout := filepath.Join(temporary, "tree")
	if _, err = gitOutput(root, "worktree", "add", "--detach", checkout, sha); err != nil {
		log.Error("Cannot create isolated checkout")
		return false
	}
	defer func() {
		if _, err := gitOutput(root, "worktree", "remove", "--force", checkout); err != nil {
			log.Error("Cannot remove isolated checkout")
		}
	}()
	head, err := gitOutput(checkout, "rev-parse", "HEAD")
	if err != nil || head != sha {
		log.Error("Checkout SHA does not match requested revision")
		return false
	}
	passed := run(checkout, relative)
	head, err = gitOutput(checkout, "rev-parse", "HEAD")
	if err != nil || head != sha {
		log.Error("Pipeline changed the checkout revision")
		return false
	}
	state := "failure"
	if passed {
		state = "success"
	}
	if err = e.Engine.Resources.RepoService.RegisterResult(services.Result{State: state, SHA: sha, Message: "Finished running pipeline"}); err != nil {
		log.Error("Failed to publish commit result")
		return false
	}
	return passed
}

// runWorktreePipeline uses a child process so relative paths, required files,
// scripts, and PWD all resolve within the tested checkout without global chdir.
func (e *Walter) runWorktreePipeline(checkout, pipeline string) bool {
	binary, err := os.Executable()
	if err != nil {
		return false
	}
	return e.runPipelineProcess(binary, checkout, pipeline)
}

// runPipelineProcess executes the same CLI in an isolated working directory.
func (e *Walter) runPipelineProcess(binary, checkout, pipeline string) bool {
	args := []string{"-mode", "local", "-c", pipeline}
	if e.Opts.StopOnAnyFailure {
		args = append(args, "-f")
	}
	for _, setting := range []struct{ source, target string }{{"stderrthreshold", "threshold"}, {"log_dir", "logDir"}} {
		if value := flag.Lookup(setting.source); value != nil && value.Value.String() != "" {
			args = append(args, "-"+setting.target, value.Value.String())
		}
	}
	command := exec.Command(binary, args...)
	command.Dir = checkout
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		log.Error(fmt.Sprintf("Pipeline process failed: %v", err))
		return false
	}
	return true
}
