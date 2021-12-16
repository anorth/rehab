package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anorth/godep/internal/db"
	"github.com/anorth/godep/internal/fetch"
	"github.com/anorth/godep/pkg/model"
)

func main() {
	skipSystemImports := flag.Bool("skip-system", false, "Skips import of system packages")

	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		fail("Usage: %s <path>\n", os.Args[0])
	}
	root := args[0]
	var opts []EachOpt
	if *skipSystemImports {
		opts = append(opts, SkipSystemImports())
	}

	// XXX: how to iterate all the packages in a module even if they are not imported by the root package

	mods, err := fetch.ListModules(root)
	if err != nil {
		fail("error listing modules: %v", err)
	}
	modules := db.NewModules(mods)

	modDeps, err := fetch.ListModuleDependencies(root)
	if err != nil {
		fail("error listing dependencies: %v", err)
	}
	modGraph := db.NewModGraph(modDeps)

	//dumpModules(modules)
	//dumpPackages(root, opts)
	//dumpDependencies(root)

	//showVersions(modGraph, "github.com/ipfs/go-cid")

	// TODO: consider skipping the main module itself, since it _is_ tested with the MVS-selected version of deps.
	q := []model.ModuleVersion{{modules.Main().Path, modules.Main().Version}}
	modulesSeen := map[string]struct{}{q[0].Module: {}}
	var node model.ModuleVersion
	for len(q) > 0 {
		node, q = q[0], q[1:]

		deps := modGraph.UpstreamOf(node.Module, node.Version)
		//fmt.Println(node, deps)
		for _, d := range deps {
			bestVersion, err := modGraph.HighestVersion(d.Upstream.Module)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "failed checking highest version of", d.Upstream.Module, err)
				continue
			}
			if d.Upstream.Version != bestVersion {
				fmt.Println(node, "depends on", d.Upstream, "but MVS selects", bestVersion)
			} else {
				// Trace through deeper in the dep graph only for the MVS-selected version of each dependency.
				// Nothing can be done about the older versions anyway.
				// TODO: tighten this further to inspect the latest version that exists (even if not selected by MVS)

				_, seen := modulesSeen[d.Upstream.Module]
				if !seen && d.Upstream.Module[:11] != "golang.org/" { // TODO: replace with whitelist
					q = append(q, d.Upstream)
					modulesSeen[d.Upstream.Module] = struct{}{}
				}
			}
		}
	}
}

func fail(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

//
//
//

func dumpModules(mods []*model.ModuleInfo) {
	for _, mod := range mods {
		fmt.Println(mod.Path, mod.Version)
	}
}

func dumpPackages(root string, opts []EachOpt) {
	gr := New()
	packages, err := fetch.ListPackages(root)
	if err != nil {
		fail("error listing dependencies: %v", err)
	}
	for _, pkg := range packages {
		//fmt.Println(pkg.ImportPath)
		//fmt.Println(pkg.Imports)
		gr.AddPackage(pkg)
	}

	// FIXME for this case, we only want to traverse packages in this module, not the whole dep tree.
	gr.EachPackage(func(pkg *model.PackageInfo) {
		fmt.Printf("%s:%s\n", pkg.ImportPath, pkg.Name)
		for _, im := range pkg.Imports {
			fmt.Printf("  %s\n", im)
		}
	}, opts...)
}

func dumpDependencies(root string) {
	deps, err := fetch.ListModuleDependencies(root)
	if err != nil {
		fail("error listing dependencies: %v", err)
	}

	for _, dep := range deps {
		fmt.Println(dep)
	}
}

func showVersions(g *db.ModGraph, upstream string) {
	on := g.DownstreamOf(upstream, "")
	for _, downstream := range on {
		fmt.Println(downstream)
	}
}

//
//
//

type Graph struct {
	// XXX is a map sufficient here? can same path point to different package infos?
	pkgs map[string]*model.PackageInfo // Import path -> package info
}

func New() *Graph {
	return &Graph{make(map[string]*model.PackageInfo)}
}

func (g *Graph) AddPackage(pkg *model.PackageInfo) {
	_, found := g.pkgs[pkg.ImportPath]
	if found {
		panic(fmt.Sprintf("duplicate package %s", pkg.ImportPath))
	}
	g.pkgs[pkg.ImportPath] = pkg
}

func (g *Graph) EachPackage(cb func(*model.PackageInfo), opts ...EachOpt) {
	resolved := resolveOpts(opts)
	for _, pkg := range g.pkgs {
		if resolved.SkipPackage(pkg) {
			continue
		}
		cb(pkg)
	}
}

type EachOpt func(opts *eachOpts)

type eachOpts struct {
	skipSystemImports bool
}

func SkipSystemImports() EachOpt {
	return func(opts *eachOpts) {
		opts.skipSystemImports = true
	}
}

func resolveOpts(opts []EachOpt) *eachOpts {
	resolved := &eachOpts{}
	for _, o := range opts {
		o(resolved)
	}
	return resolved
}

func (o *eachOpts) SkipPackage(p *model.PackageInfo) bool {
	return o.skipSystemImports && p.Standard
}

// Returns the keys from a string map.
func keys(pkgImports map[string]struct{}) []string {
	var k []string
	for i, _ := range pkgImports {
		k = append(k, i)
	}
	return k
}
