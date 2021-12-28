package remote

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"regexp"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var ghRepoRe = regexp.MustCompile("github.com/([\\w-]+)/([\\w-]+)(/.*)?")

type Remote struct {
	client *github.Client
	info   *github.Repository
}

func Open(ctx context.Context, path, token string) (*Remote, error) {
	match := ghRepoRe.FindStringSubmatch(path)
	if len(match) == 0 {
		return nil, fmt.Errorf("%s isn't a GitHub repo path", path)
	}
	owner, repo := match[1], match[2]

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	repoInfo, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed fetching repo %s/%s: %w", owner, repo, err)
	}


	return &Remote{
		client: client,
		info: repoInfo,
	}, nil
}

func (r *Remote) URL() string {
	return r.info.GetURL()
}

func (r *Remote) ID() (owner, repo string) {
	return r.info.GetOwner().GetLogin(), r.info.GetName()
}


// Pushes a commit editing a single file.
// Returns the commit SHA, or "" if the file isn't modified.
func (r *Remote) EditFile(ctx context.Context, name string, edit func([]byte) ([]byte, error)) (string, error) {
	log.Printf("fetching head commit for %s", r.URL())
	owner, repo := r.ID()
	commits, _, err := r.client.Repositories.ListCommits(ctx, owner, repo, nil)
	if err != nil {
		return "", fmt.Errorf("failed listing commits for %s: %w", r.URL(), err)
	}
	head := commits[0]
	log.Printf("head at %s by %s", head.GetSHA(), head.GetAuthor().GetLogin())
	tree, _, err := r.client.Git.GetTree(ctx, owner, repo, head.GetSHA(), false)
	if err != nil {
		return "", fmt.Errorf("failed fetching tree for %s at %s: %w", r.URL(), head.GetSHA(), err)
	}

	var fileEntry *github.TreeEntry
	for i := range tree.Entries {
		if tree.Entries[i].GetPath() == name && tree.Entries[i].GetType() == "blob" {
			fileEntry = &tree.Entries[i]
			//log.Println(fileEntry.String())
			break
		}
	}
	if fileEntry == nil {
		return "", fmt.Errorf("no file %s in repo root %s", name, r.URL())
	}

	//log.Printf("fetching blob for %s", fileEntry.GetPath())
	fileBlob, _, err := r.client.Git.GetBlob(ctx, owner, repo, fileEntry.GetSHA())
	if err != nil {
		return "", fmt.Errorf("failed fetching blob for %s in %s at %s: %w", name, r.URL(), head.GetSHA(), err)
	}
	content := fileBlob.GetContent()
	b64 := base64.StdEncoding
	decoded := make([]byte, b64.DecodedLen(len(content)))
	decodedLen, err := b64.Decode(decoded, []byte(content))
	if err != nil {
		return "", fmt.Errorf("failed decoding go.mod content: %w", err)
	}

	modifiedContent, err := edit(decoded[:decodedLen])
	if err != nil {
		return "", fmt.Errorf("failed editing file %s in %s at %s: %w", name, r.URL(), head.GetSHA(), err)
	}

	if bytes.Equal(decoded[:decodedLen], modifiedContent) {
		return "", nil
	}

	// Push new file as a commit
	newContentString := string(modifiedContent)
	newEntry := github.TreeEntry{
		Path:    fileEntry.Path,
		Mode:    fileEntry.Mode,
		Type:    fileEntry.Type,
		Content: &newContentString,
	}
	//log.Printf("pushing new blob for %s", fileEntry.GetPath())
	newTree, _, err := r.client.Git.CreateTree(ctx, owner, repo, tree.GetSHA(), []github.TreeEntry{newEntry})
	if err != nil {
		// This will fail with request status code 404 if token lacks push permission.
		return "", fmt.Errorf("failed to create new tree for %s: %w", r.URL(), err)
	}
	//log.Printf("new tree %+v", newTree)

	message := "Upgrade module requirements"
	newCommit := github.Commit{
		//Author:       nil,
		//Committer:    nil,
		Message: &message,
		Tree:    newTree,
		Parents: []github.Commit{{SHA: head.SHA}},
	}
	log.Printf("pushing commit for tree %s", newTree.GetSHA())
	commit, _, err := r.client.Git.CreateCommit(ctx, owner, repo, &newCommit)
	if err != nil {
		return "",  fmt.Errorf("failed to commit new tree for %s: %w", r.URL(), err)
	}
	log.Printf("pushed commit %s", commit.GetSHA())
	return commit.GetSHA(), nil
}

func (r *Remote) MakeBranch(ctx context.Context, commitSHA, name string) (string, error) {
	owner, repo := r.ID()
	refName := "refs/heads/" + name
	ref := github.Reference{
		Ref: &refName,
		Object: &github.GitObject{
			SHA: &commitSHA,
		},
	}
	log.Printf("pushing branch %s at %s", refName, commitSHA)
	_, _, err := r.client.Git.CreateRef(ctx, owner, repo, &ref)
	if err != nil {
		if gherr := err.(*github.ErrorResponse); gherr != nil {
			if gherr.Message == "Reference already exists" {
				err = nil
				// FIXME update the ref, or use a unique ref name
			}
		}
	}
	if err != nil {
		return "", fmt.Errorf("failed to push ref %s %s for %s: %w", refName, commitSHA, r.URL(), err)
	}
	return refName, nil
}

func (r *Remote) CompareBranch(refName, title, message string) (string, error) {
	owner, repo := r.ID()
	compareURL := fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s?title=%s&body=%s",
		owner, repo, r.info.GetDefaultBranch(), refName, url.QueryEscape(title), url.QueryEscape(message))
	return compareURL, nil
}

func (r *Remote) MakePull(ctx context.Context, refName, title, message string) (string, error) {
	owner, repo := r.ID()
	newPull := github.NewPullRequest{
		Title: &title,
		Head:  &refName,
		Base:  r.info.DefaultBranch,
		Body:  &message,
	}
	log.Printf("making pull request for %s on %s", newPull.GetHead(), newPull.GetBase())
	pull, _, err := r.client.PullRequests.Create(ctx, owner, repo, &newPull)
	if err != nil {
		return "", fmt.Errorf("failed making pull request ref %s for %s: %w", refName, r.URL(), err)
	}
	return pull.GetURL(), nil
}