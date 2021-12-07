package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	skipTestPackages := flag.Bool("skip-test-pkg", false, "Skips packages ending in _test")
	skipTestFiles := flag.Bool("skip-test-file", false, "Skips files ending in _test")
	skipSystemImports := flag.Bool("skip-system", false, "Skips import of system packages")

	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		fail("Usage: %s <path>\n", os.Args[0])
	}
	root := args[0]

	gr := New()
	acceptAll := func(info os.FileInfo) bool { return true }
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}
		if len(info.Name()) > 1 && info.Name()[0] == '.' {
			return filepath.SkipDir
		}

		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, path, acceptAll, 0)
		if err != nil {
			fail("error parsing %s: %v\n", path, err)
		}

		for _, pkg := range pkgs {
			for fileName, f := range pkg.Files {
				gr.AddFile(Package{Path: path, Name: pkg.Name}, fileName)
				for _, m := range f.Imports {
					gr.AddImport(fileName, m.Path.Value)
				}
			}
		}

		return nil
	})
	if err != nil {
		fail("error walking %q: %v\n", root, err)
	}

	var opts []EachOpt
	if *skipTestPackages {
		opts = append(opts, SkipTestPackages())
	}
	if *skipTestFiles {
		opts = append(opts, SkipTestFiles())
	}
	if *skipSystemImports {
		opts = append(opts, SkipSystemImports())
	}
	//gr.EachFile(func(path string, imports []string) {
	//	fmt.Println(path)
	//	for _, im := range imports {
	//		fmt.Printf("  %s\n", im)
	//	}
	//}, opts...)

	gr.EachPackage(func(pkg Package, imports []string) {
		fmt.Printf("%s:%s\n", pkg.Path, pkg.Name)
		for _, im := range imports {
			fmt.Printf("  %s\n", im)
		}
	}, opts...)
}

func fail(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

//
//
//

type Package struct {
	Path string
	Name string
}

type Graph struct {
	pkgs     []Package
	pkgFiles map[Package][]string // Package -> file paths
	imports  map[string][]string  // File path -> import paths
}

func New() *Graph {
	return &Graph{nil, make(map[Package][]string), make(map[string][]string)}
}

func (g *Graph) AddFile(pkg Package, filePath string) {
	_, fileFound := g.imports[filePath]
	if fileFound {
		panic(fmt.Sprintf("Duplicate file %s", filePath))
	}
	_, pkgFound := g.pkgFiles[pkg]
	if !pkgFound {
		g.pkgs = append(g.pkgs, pkg)
	}
	g.pkgFiles[pkg] = append(g.pkgFiles[pkg], filePath)
	g.imports[filePath] = []string{}
}

func (g *Graph) AddImport(filePath string, importPath string) {
	_, found := g.imports[filePath]
	if !found {
		panic(fmt.Sprintf("Unknown file %s", filePath))
	}
	g.imports[filePath] = append(g.imports[filePath], importPath)
}

func (g *Graph) EachPackage(cb func(Package, []string), opts ...EachOpt) {
	resolved := resolveOpts(opts)
	for _, pkg := range g.pkgs {
		if resolved.SkipPackage(&pkg) {
			continue
		}
		// Accumulate imports from all files
		pkgImports := make(map[string]struct{})
		for _, f := range g.pkgFiles[pkg] {
			if resolved.SkipFile(f) {
				continue
			}
			for _, i := range g.imports[f] {
				pkgImports[i] = struct{}{}
			}
		}
		unique := resolved.FilterImports(keys(pkgImports))
		sort.Slice(unique, func(i, j int) bool {
			// Sort by system imports, then lexicographically.
			if isSystemImport(unique[i]) && !isSystemImport(unique[j]) {
				return true
			} else if !isSystemImport(unique[i]) && isSystemImport(unique[j]) {
				return false
			}
			return strings.Compare(unique[i], unique[j]) < 0
		})
		cb(pkg, unique)
	}
}

func (g *Graph) EachFile(cb func(string, []string), opts ...EachOpt) {
	resolved := resolveOpts(opts)
	for _, pkg := range g.pkgs {
		if resolved.SkipPackage(&pkg) {
			continue
		}
		for _, f := range g.pkgFiles[pkg] {
			if resolved.SkipFile(f) {
				continue
			}
			cb(f, resolved.FilterImports(g.imports[f][:]))
		}
	}
}

type EachOpt func(opts *eachOpts)

type eachOpts struct {
	skipTestPackages  bool
	skipTestFiles     bool
	skipSystemImports bool
}

func SkipTestPackages() EachOpt {
	return func(opts *eachOpts) {
		opts.skipTestPackages = true
	}
}

func SkipTestFiles() EachOpt {
	return func(opts *eachOpts) {
		opts.skipTestFiles = true
	}
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

func (o *eachOpts) SkipPackage(p *Package) bool {
	return o.skipTestPackages && strings.HasSuffix(p.Name, "_test")
}

func (o *eachOpts) SkipFile(name string) bool {
	return o.skipTestFiles && strings.HasSuffix(name, "_test")
}

func (o *eachOpts) FilterImports(imports []string) []string {
	if !o.skipSystemImports {
		return imports
	}
	var filtered []string
	for _, im := range imports {
		if !isSystemImport(im) {
			filtered = append(filtered, im)
		}
	}
	return filtered
}

// Hacky check for system imports
func isSystemImport(path string) bool {
	count := strings.Count(path, "/")
	return count < 2
}

// Returns the keys from a string map.
func keys(pkgImports map[string]struct{}) []string {
	var k []string
	for i, _ := range pkgImports {
		k = append(k, i)
	}
	return k
}
