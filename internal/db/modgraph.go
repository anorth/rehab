package db

import (
	"fmt"

	"github.com/anorth/rehab/pkg/model"
	"golang.org/x/mod/semver"
)

// A module dependency graph data model.
type ModGraph struct {
	edges []model.ModuleRelationship // unordered
	// TODO: indexes
}

func NewModGraph(rels []model.ModuleRelationship) *ModGraph {
	g := &ModGraph{}
	g.edges = append(g.edges, rels...) // copy
	return g
}

func (g *ModGraph) Edges() []model.ModuleRelationship {
	return g.edges[:]
}

// Finds all upstream dependencies of a query module (optionally: at some version).
func (g *ModGraph) UpstreamOf(modPath, version string) []model.ModuleRelationship {
	var result []model.ModuleRelationship
	for _, e := range g.edges {
		if e.Downstream.Path == modPath && (version == "" || e.Downstream.Version == version) {
			result = append(result, e)
		}
	}
	return result
}

// Finds all downstream dependencies on a query module (optionally: at some version).
func (g *ModGraph) DownstreamOf(modPath string, version string) []model.ModuleRelationship {
	var result []model.ModuleRelationship
	for _, e := range g.edges {
		if e.Upstream.Path == modPath && (version == "" || e.Upstream.Version == version){
			result = append(result, e)
		}
	}
	return result
}

// Returns the highest version of a module required by the graph, with one of the modules that explicitly
// requires it.
func (g *ModGraph) SelectedVersion(modPath string) (string, model.ModuleVersion, error) {
	var result string
	var reason model.ModuleVersion
	for _, e := range g.edges {
		if e.Upstream.Path == modPath {
			if result == "" || semver.Compare(result, e.Upstream.Version) < 0 {
				result = e.Upstream.Version
				reason = e.Downstream
			}
		}
	}
	if result == "" {
		return result, reason, fmt.Errorf("no dependencies on %s", modPath)
	}
	return result, reason, nil
}
