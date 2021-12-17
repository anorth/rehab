package model

import (
	"fmt"
	"strings"
)

// A module path and version name.
type ModuleVersion struct {
	Module  string
	Version string
}

// Formats the module version in the same way as `go mod graph`.
func (mv ModuleVersion) String() string {
	if mv.Version != "" {
		return fmt.Sprintf("%s@%s", mv.Module, mv.Version)
	}
	return mv.Module
}

func (mv *ModuleVersion) Parse(s string) error {
	ss := strings.Split(s, "@")
	switch len(ss) {
	case 2:
		mv.Module, mv.Version = ss[0], ss[1]
	case 1:
		mv.Module, mv.Version = ss[0], ""
	default:
		return fmt.Errorf("bad module version, too many '@'")
	}
	return nil
}

// A dependency relationship between two module versions.
type ModuleRelationship struct {
	Downstream ModuleVersion // consumer
	Upstream   ModuleVersion // dependency
}

func (r ModuleRelationship) String() string {
	return fmt.Sprintf("%s %s", r.Downstream, r.Upstream)
}