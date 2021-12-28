package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"regexp"

	"github.com/anorth/rehab/internal/db"
	"github.com/anorth/rehab/internal/fetch"
	"github.com/anorth/rehab/pkg/model"
	"github.com/google/go-github/github"
	"golang.org/x/mod/modfile"
	"golang.org/x/oauth2"
)

var GH_REPO_RE = regexp.MustCompile("github.com/([\\w-]+)/([\\w-]+)(/.*)?")

type Rehab struct {
}

func (app *Rehab) Show(root string, all bool) error {
	// TODO: short-circuit the traversal if not showing all
	modules, err := app.fetchModules(root)
	if err != nil {
		return err
	}
	modGraph, err := app.fetchModGraph(root)
	if err != nil {
		return err
	}
	mainModule := modules.Main()

	stale := FindStaleVersions(modules, modGraph)
	for _, s := range stale {
		if s.Consumer.Module == mainModule.Path || all {
			fmt.Println(s)
		}
	}
	return nil
}

func (app *Rehab) Propose(ctx context.Context, root string, all bool) error {
	// TODO: short-circuit the traversal if not showing all
	// TODO: option to upgrade to MVS-selected, instead of highest.
	modules, err := app.fetchModules(root)
	if err != nil {
		return err
	}
	modGraph, err := app.fetchModGraph(root)
	if err != nil {
		return err
	}
	mainModule := modules.Main()

	stale := FindStaleVersions(modules, modGraph)
	// Proposed requirement upgrades keyed by consuming module
	upgrades := map[string][]model.ModuleVersion{}
	for _, s := range stale {
		if s.Consumer.Module == mainModule.Path || all {
			upgrades[s.Consumer.Module] = append(upgrades[s.Consumer.Module], model.ModuleVersion{
				Module:  s.Requirement.Module,
				Version: s.HighestVersion,
			})
		}
	}

	for modPath, reqs := range upgrades {
		module, err := modules.ForPath(modPath)
		if err != nil {
			return err // TODO wrap
		}
		if err := app.proposeUpgrade(ctx, module, reqs); err != nil {
			return err // TODO wrap
		}
	}

	return nil

}

///// Private implementation /////

func (app *Rehab) fetchModules(root string) (*db.Modules, error) {
	mods, err := fetch.ListModules(root)
	if err != nil {
		return nil, fmt.Errorf("error listing modules: %w", err)
	}
	return db.NewModules(mods), nil
}

func (app *Rehab) fetchModGraph(root string) (*db.ModGraph, error) {
	modDeps, err := fetch.ListModuleDependencies(root)
	if err != nil {
		return nil, fmt.Errorf("error listing dependencies: %w", err)
	}
	return db.NewModGraph(modDeps), nil
}

func (app *Rehab) fetchPackages(root string) ([]*model.PackageInfo, error) {
	packages, err := fetch.ListPackages(root)
	if err != nil {
		return nil, fmt.Errorf("error listing dependencies: %w", err)
	}
	return packages, nil
}

func (app *Rehab) proposeUpgrade(ctx context.Context, module *model.ModuleInfo, reqs []model.ModuleVersion) error {
	log.Printf("upgrading %s", module.Path)

	// Find the module repo on GitHub

	match := GH_REPO_RE.FindStringSubmatch(module.Path)
	if len(match) == 0 {
		return fmt.Errorf("can't upgrade %s, not GitHub", module.Path)
	}
	owner, repo := match[1], match[2]
	log.Printf("owner: %s, repo %s", owner, repo)

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "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"}, // FIXME use env/arg
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	repoInfo, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed fetching repo %s/%s: %w", owner, repo, err)
	}
	repoURL := repoInfo.GetURL()
	log.Printf("repo %s", repoURL)

	// Fetch go.mod file (get current commit, retrieve tree, retrieve blob)
	log.Printf("fetching head commit for %s", repoURL)
	commits, _, err := client.Repositories.ListCommits(ctx, owner, repo, nil)
	if err != nil {
		return fmt.Errorf("failed listing commits for %s: %w", repoURL, err)
	}
	head := commits[0]
	log.Printf("head at %s by %s", head.GetSHA(), head.GetAuthor().GetLogin())
	tree, _, err := client.Git.GetTree(ctx, owner, repo, head.GetSHA(), false)
	if err != nil {
		return fmt.Errorf("failed fetching tree for %s at %s: %w", repoURL, head.GetSHA(), err)
	}

	var goModEntry *github.TreeEntry
	for i := range tree.Entries {
		// XXX How to find go.mod file when it's not in the root, like for nested modules?
		if tree.Entries[i].GetPath() == "go.mod" && tree.Entries[i].GetType() == "blob" {
			goModEntry = &tree.Entries[i]
			log.Println(goModEntry.String())
			break
		}
	}
	if goModEntry == nil {
		return fmt.Errorf("no go.mod file in repo root %s", repoInfo.URL)
	}

	log.Printf("fetching %s", goModEntry.GetPath())
	goModBlob, _, err := client.Git.GetBlob(ctx, owner, repo, goModEntry.GetSHA())
	if err != nil {
		return fmt.Errorf("failed fetching go.mod blob for %s at %s: %w", repoURL, head.GetSHA(), err)
	}
	content := goModBlob.GetContent()
	b64 := base64.StdEncoding
	decoded := make([]byte, b64.DecodedLen(len(content)))
	decodedLen, err := b64.Decode(decoded, []byte(content))
	if err != nil {
		return fmt.Errorf("failed decoding go.mod content: %w", err)
	}
	modFile, err := modfile.Parse("go.mod", decoded[:decodedLen], nil)
	if err != nil {
		return fmt.Errorf("failed parsing go.mod for %s at %s: %w", repoURL, head.GetSHA(), err)
	}

	// Replace go.mod file lines
	modified := false
	for _, req := range reqs {
		err = modFile.AddRequire(req.Module, req.Version) // Updates requirement in-place, preserving comments.
		if err != nil {
			log.Printf("failed to add requirement %s: %w", req, err)
			continue
		}
		modified = true
	}

	if !modified {
		log.Printf("go.mod not modified")
		return nil
	}

	newGoModContent, err := modFile.Format()
	if err != nil {
		return fmt.Errorf("failed to format new go.mod file for %s: %w", repoURL, err)
	}

	// Push new file as a commit
	newGoModString := string(newGoModContent)
	newEntry := github.TreeEntry{
		Path:    goModEntry.Path,
		Mode:    goModEntry.Mode,
		Type:    goModEntry.Type,
		Content: &newGoModString,
	}
	log.Printf("pushing new blob for %s", goModEntry.GetPath())
	newTree, _, err := client.Git.CreateTree(ctx, owner, repo, tree.GetSHA(), []github.TreeEntry{newEntry})
	if err != nil {
		return fmt.Errorf("failed to create new tree for %s: %w", repoURL, err)
	}
	log.Printf("new tree %+v", newTree)

	message := "Update deps FIXME"
	newCommit := github.Commit{
		//Author:       nil,
		//Committer:    nil,
		Message: &message,
		Tree:    newTree,
		Parents: []github.Commit{{SHA: head.SHA}},
	}
	log.Printf("pushing commit")
	commit, _, err := client.Git.CreateCommit(ctx, owner, repo, &newCommit)
	if err != nil {
		return fmt.Errorf("failed to commit new tree for %s: %w", repoURL, err)
	}
	log.Printf("new commit %+v", commit)

	// Create a branch pointing at the commit
	refName := "refs/heads/rehab/fixme"
	ref := github.Reference{
		Ref: &refName,
		Object: &github.GitObject{
			SHA: commit.SHA,
		},
	}
	log.Printf("pushing branch %s at %s", refName, commit.GetSHA())
	_, _, err = client.Git.CreateRef(ctx, owner, repo, &ref)
	if err != nil {
		if gherr := err.(*github.ErrorResponse); gherr != nil {
			if gherr.Message == "Reference already exists" {
				err = nil
				// FIXME update the ref, or use a unique ref name
			}
		}
	}
	if err != nil {
		return fmt.Errorf("failed to push ref %s %s for %s: %w", refName, commit.GetSHA(), repoURL, err)
	}

	// Create a PR
	title := "Update deps FIXME"
	body := "body here"
	newPull := github.NewPullRequest{
		Title: &title,
		Head:  &refName,
		Base:  repoInfo.DefaultBranch,
		Body:  &body,
	}
	log.Printf("making pull request %+v", newPull)
	pull, _, err := client.PullRequests.Create(ctx, owner, repo, &newPull)
	if err != nil {
		return fmt.Errorf("failed making pull request ref %s %s for %s: %w", refName, commit.GetSHA(), repoURL, err)
	}

	log.Printf("PR: %s", pull.GetURL())
	return nil
}

func dumpModules(modules *db.Modules) {
	for _, mod := range modules.All() {
		fmt.Println(mod.Path, mod.Version)
	}
}

func dumpPackages(packages []*model.PackageInfo) {
	for _, pkg := range packages {
		fmt.Printf("%s:%s\n", pkg.ImportPath, pkg.Name)
		for _, im := range pkg.Imports {
			fmt.Printf("  %s\n", im)
		}
	}
}

func dumpRelationships(g *db.ModGraph) {
	for _, dep := range g.Edges() {
		fmt.Println(dep)
	}
}
