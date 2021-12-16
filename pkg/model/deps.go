package model

import (
	"fmt"
	"strings"
)

type ModuleVersion struct {
	Module  string
	Version string
}

func (mv ModuleVersion) String() string {
	return fmt.Sprintf("%s@%s", mv.Module, mv.Version)
}

func (mv *ModuleVersion) Parse(s string) error {
	ss := strings.Split(s, "@")
	switch len(ss) {
	case 2:
		mv.Module, mv.Version = ss[0], ss[1]
	case 1:
		mv.Module, mv.Version = ss[0], "tip"
	default:
		return fmt.Errorf("bad module version, too many '@'")
	}
	return nil
}



type ModuleRelationship struct {
	Downstream ModuleVersion // consumer
	Upstream   ModuleVersion // dependency
}

func (r ModuleRelationship) String() string {
	return fmt.Sprintf("%s %s", r.Downstream, r.Upstream)
}