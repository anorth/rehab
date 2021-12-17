package db

import (
	"fmt"

	"github.com/anorth/rehab/pkg/model"
)

// A module database.
type Modules struct {
	modules []*model.ModuleInfo // Main module is first, otherwise unordered
	// TODO: indexes
}

func NewModules(modules []*model.ModuleInfo) *Modules {
	return &Modules{modules}
}

func (m *Modules) All() []*model.ModuleInfo {
	return m.modules[:]
}

func (m *Modules) Main() *model.ModuleInfo {
	return m.modules[0]
}

func (m *Modules) ForPath(path string) (*model.ModuleInfo, error) {
	for _, mod := range m.modules {
		if mod.Path == path {
			return mod, nil
		}
	}
	return nil, fmt.Errorf("no module with path %s", path)
}
