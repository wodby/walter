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

// Package services provides the functionality for all supported services (GitHub)
package services

import (
	"container/list"
	"context"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/google/go-github/v92/github"
	"github.com/wodby/walter/log"
)

// GitHubClient struct
type GitHubClient struct {
	Repo         string `config:"repo"`
	From         string `config:"from"`
	Token        string `config:"token"`
	UpdateFile   string `config:"update"`
	TargetBranch string `config:"branch"`
	BaseUrl      *url.URL
}

// GetUpdateFilePath returns the update file name
func (githubClient *GitHubClient) GetUpdateFilePath() string {
	if githubClient.UpdateFile != "" {
		return githubClient.UpdateFile
	}
	return DefaultUpdateFileName

}

// RegisterResult registers the supplied result
func (githubClient *GitHubClient) RegisterResult(result Result) error {
	client, err := githubClient.newClient()
	if err != nil {
		return err
	}

	log.Info("Submitting result")
	repositories := client.Repositories
	status, _, err := repositories.CreateStatus(context.Background(),
		githubClient.From,
		githubClient.Repo,
		result.SHA,
		github.RepoStatus{
			State:       github.String(result.State),
			TargetURL:   github.String(result.Url),
			Description: github.String(result.Message),
			Context:     github.String("continuous-integraion/walter"),
		})
	log.Infof("Submit status: %s", status)
	if err != nil {
		log.Errorf("Failed to register result: %s", err)
	}
	return err
}

// GetCommits get a list of all the commits for the current update
func (githubClient *GitHubClient) GetCommits(update Update) (*list.List, error) {
	log.Info("getting commits\n")
	commits := list.New()
	client, err := githubClient.newClient()
	if err != nil {
		return nil, err
	}

	// get a list of pull requests with Pull Request API
	pullreqs, _, err := client.PullRequests.List(context.Background(),
		githubClient.From, githubClient.Repo,
		&github.PullRequestListOptions{})
	if err != nil {
		log.Errorf("Failed to get pull requests")
		return list.New(), err
	}

	re, err := regexp.Compile(githubClient.TargetBranch)
	if err != nil {
		log.Error("Failed to compile branch pattern...")
		return list.New(), err
	}

	log.Infof("Size of pull reqests: %d", len(pullreqs))
	for _, pullreq := range pullreqs {
		if pullreq.Head == nil || pullreq.Head.Ref == nil || pullreq.State == nil || pullreq.Number == nil || pullreq.UpdatedAt == nil || pullreq.Head.SHA == nil {
			continue
		}
		log.Infof("Branch name is \"%s\"", *pullreq.Head.Ref)

		if githubClient.TargetBranch != "" {
			matched := re.Match([]byte(*pullreq.Head.Ref))
			if matched != true {
				log.Infof("Not add a branch, \"%s\" since this branch name is not match the filtering pattern", *pullreq.Head.Ref)
				continue
			}
		}

		if *pullreq.State == "open" && pullreq.UpdatedAt.After(update.Time) {
			log.Infof("Adding pullrequest %d", *pullreq.Number)
			commits.PushBack(*pullreq)
		}
	}

	// get the latest commit with Commit API if the commit is newer than last update
	masterCommits, _, err := client.Repositories.ListCommits(context.Background(),
		githubClient.From, githubClient.Repo, &github.CommitsListOptions{})
	if err != nil {
		return nil, err
	}
	if len(masterCommits) > 0 && masterCommits[0].GetCommit().GetAuthor().GetDate().After(update.Time) {
		commits.PushBack(*masterCommits[0])
	}
	return commits, nil
}

// newClient confines authenticated requests to the configured endpoint.
func (c *GitHubClient) newClient() (*github.Client, error) {
	options := []github.ClientOptionsFunc{github.WithHTTPClient(&http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	})}
	if c.Token != "" {
		options = append(options, github.WithAuthToken(c.Token))
	}
	if c.BaseUrl != nil {
		base := c.BaseUrl.String()
		options = append(options, github.WithURLs(&base, nil))
	}
	return github.NewClient(options...)
}
