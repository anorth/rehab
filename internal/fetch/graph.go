package fetch

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/anorth/godep/pkg/model"
)

// Lists the module requirement graph of the module at modulePath.
// The main module at modulePath is returned first.
// See https://go.dev/ref/mod#go-mod-graph
func ListModuleDependencies(modulePath string) ([]model.ModuleRelationship, error) {
	raw, err := Exec(modulePath, "go", "mod", "graph")
	if err != nil {
		return nil, fmt.Errorf("failed listing dependencies for %s: %w", modulePath, err)
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(raw))
	var result []model.ModuleRelationship
	for scanner.Scan() {
		line := scanner.Text()
		hunks := strings.Split(line, " ")
		if len(hunks) != 2 {
			return nil, fmt.Errorf("bad mod graph line: '%s'", line)
		}
		result = append(result, model.ModuleRelationship{})
		r := &result[len(result) - 1]
		if err := r.Downstream.Parse(hunks[0]); err != nil {
			return nil, err
		}
		if err := r.Upstream.Parse(hunks[1]); err != nil {
			return nil, err
		}
	}
	return result, nil
}
