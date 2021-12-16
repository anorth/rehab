package model

import "time"

// ModuleInfo is the data returned by 'go list -m --json' for a Go module.
// Derived from golang.org/x/tools/go/packages.
type ModuleInfo struct {
	Path      string       // module path
	Version   string       // module version
	Versions  []string     // available module versions (with -versions)
	Replace   *ModuleInfo  // replaced by this module
	Time      *time.Time   // time version was created
	Update    *ModuleInfo  // available update, if any (with -u)
	Main      bool         // is this the main module?
	Indirect  bool         // is this module only an indirect dependency of main module?
	Dir       string       // directory holding files for this module, if any
	GoMod     string       // path to go.mod file used when loading this module, if any
	GoVersion string       // go version used in module
	Retracted string       // retraction information, if any (with -retracted or -u)
	Error     *ModuleError // error loading module}
}

type ModuleError struct {
	Err string // the error itself
}