package main

import (
	"fmt"
	"os"

	"github.com/anorth/rehab/internal/action"
	"github.com/anorth/rehab/internal/db"
	"github.com/anorth/rehab/internal/fetch"
	"github.com/anorth/rehab/pkg/model"
)

func main() {
	if len(os.Args) != 2 {
		fail("Usage: %s <path>\n", os.Args[0])
	}
	root := os.Args[1]

	mods, err := fetch.ListModules(root)
	if err != nil {
		fail("error listing modules: %v", err)
	}
	modules := db.NewModules(mods)
	//dumpModules(modules)

	modDeps, err := fetch.ListModuleDependencies(root)
	if err != nil {
		fail("error listing dependencies: %v", err)
	}
	modGraph := db.NewModGraph(modDeps)
	//dumpRelationships(modGraph)

	//packages, err := fetch.ListPackages(root)
	//if err != nil {
	//	fail("error listing dependencies: %v", err)
	//}
	//dumpPackages(packages)

	stale := action.FindStaleVersions(modules, modGraph)
	for _, s := range stale {
		fmt.Println(s)
		//fmt.Println(node, "depends on", d.Upstream, "but MVS selects", bestVersion)
	}
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

func fail(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}
