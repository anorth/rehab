package cmd

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/anorth/rehab/internal/db"
	"github.com/anorth/rehab/internal/fetch"
	"github.com/anorth/rehab/internal/remote"
	"github.com/anorth/rehab/pkg/model"
	"golang.org/x/mod/modfile"
)

type Rehab struct {
	GitHubToken      string // GitHub authentication token
	MinimumUpgrade   bool   // Restrict upgrades to MVS-selected version, rather than latest
	BranchPrefix     string // Prefix for branches pushed to GitHub
	MakePullRequests bool   // Initiate pull requests (rather than only pushing branches)
	Verbose          bool   // Whether to log progress
}

func (app *Rehab) Show(root string, all bool) error {
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
	sort.Slice(stale, func(i, j int) bool {
		return strings.Compare(stale[i].Consumer.Module, stale[j].Consumer.Module) < 0
	})
	for _, s := range stale {
		if s.Consumer.Module == mainModule.Path || all {
			fmt.Println(s)
		}
	}
	return nil
}

func (app *Rehab) Propose(ctx context.Context, root string, all bool) error {
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
			upgradeTo := s.HighestVersion
			if app.MinimumUpgrade {
				upgradeTo = s.SelectedVersion
			}
			upgrades[s.Consumer.Module] = append(upgrades[s.Consumer.Module], model.ModuleVersion{
				Module:  s.Requirement.Module,
				Version: upgradeTo,
			})
		}
	}

	for modPath, reqs := range upgrades {
		module, err := modules.ForPath(modPath)
		if err != nil {
			return err
		}
		pullURL, err := app.proposeUpgrade(ctx, module, app.GitHubToken, reqs)
		if err != nil {
			// Keep trying other modules (the error may be a missing push permission).
			log.Printf("failed upgrading %s: %s", module.Path, err)
			continue
		} else if pullURL == ""  {
			fmt.Println("No changes for", module.Path)
			continue
		}
		fmt.Println("Pull request at", pullURL)
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

// Returns URL to a PR or comparison, or "" if no changes made.
func (app *Rehab) proposeUpgrade(ctx context.Context, module *model.ModuleInfo, token string, reqs []model.ModuleVersion) (url string, err error) {
	log.Printf("upgrading requirements for %s", module.Path)
	repo, err := remote.Open(ctx, module.Path, token)

	// FIXME Find go.mod file when it's not in the root, like for nested modules
	commitSHA, err := repo.EditFile(ctx, "go.mod", func(original []byte) ([]byte, error) {
		modFile, err := modfile.Parse("go.mod", original, nil)
		if err != nil {
			return nil, fmt.Errorf("failed parsing go.mod: %w", err)
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
			return original, nil
		}

		newContent, err := modFile.Format()
		if err != nil {
			return nil, fmt.Errorf("failed to format new go.mod file: %w", err)
		}
		return newContent, nil
	})
	if err != nil {
		return "", err
	} else if commitSHA == "" {
		return "", nil
	}

	// Create a branch pointing at the commit
	refName, err := repo.MakeBranch(ctx, commitSHA, app.BranchPrefix+"upgrade")
	if err != nil {
		return "", err
	}

	title := "Update module requirements"
	body := "Upgrades to the latest version of requirements.\n\n" +
		"This is an automated PR created by Rehab."
	if app.MakePullRequests {
		pullURL, err := repo.MakePull(ctx, refName, title, body)
		if err != nil {
			return "", err
		}
		return pullURL, nil
	} else {
		compareURL, err := repo.CompareBranch(refName, title, body)
		if err != nil {
			return "", err
		}
		return compareURL, nil
	}
}

//func dumpModules(modules *db.Modules) {
//	for _, mod := range modules.All() {
//		fmt.Println(mod.Path, mod.Version)
//	}
//}
//
//func dumpPackages(packages []*model.PackageInfo) {
//	for _, pkg := range packages {
//		fmt.Printf("%s:%s\n", pkg.ImportPath, pkg.Name)
//		for _, im := range pkg.Imports {
//			fmt.Printf("  %s\n", im)
//		}
//	}
//}
//
//func dumpRelationships(g *db.ModGraph) {
//	for _, dep := range g.Edges() {
//		fmt.Println(dep)
//	}
//}
