package action

import (
	"fmt"
	"os"

	"github.com/anorth/godep/internal/db"
	"github.com/anorth/godep/pkg/model"
)

type StaleVersion struct {
	Consumer        model.ModuleVersion // The module version requiring an old upstream dependency
	Requirement     model.ModuleVersion // The module required and declared version
	SelectedVersion string              // The version selected by MVS
	SelectedReason  model.ModuleVersion // A (transitively) required module that declares the selected version requirement
	HighestVersion  string              // The highest available version of the requirement
}

// Finds imports in a dependency tree based at some root module where a declared dependency module version
// does not match the module version chosen by MVS when building the root module.
// This situation means that the tests for the consuming module run with a different version of the
// dependency than that actually used in production.
func FindStaleVersions(modules *db.Modules, modGraph *db.ModGraph) []*StaleVersion {
	// TODO: consider skipping the main module itself, since it _is_ tested with the MVS-selected version of deps.
	var found []*StaleVersion
	q := []model.ModuleVersion{{modules.Main().Path, modules.Main().Version}}
	modulesSeen := map[string]struct{}{q[0].Module: {}}
	var node model.ModuleVersion
	for len(q) > 0 {
		node, q = q[0], q[1:]
		deps := modGraph.UpstreamOf(node.Module, node.Version)
		//fmt.Println(node, deps)
		for _, d := range deps {
			selected, reason, err := modGraph.HighestVersion(d.Upstream.Module)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "failed checking highest version of", d.Upstream.Module, err)
				continue
			}
			if d.Upstream.Version != selected {
				//fmt.Println(node, "depends on", d.Upstream, "but MVS selects", selected)
				selectedInfo, err := modules.ForPath(d.Upstream.Module)
				if err != nil {
					_, _ = fmt.Fprintln(os.Stderr, "failed loading module of", d.Upstream.Module, err)
					continue
				}
				highestVersion := selectedInfo.Version
				for _, v := range selectedInfo.Versions {
					highestVersion = v // XXX are they in increasing order?
				}
				found = append(found, &StaleVersion{
					Consumer:        node,
					Requirement:     d.Upstream,
					SelectedVersion: selected,
					SelectedReason:  reason,
					HighestVersion:  highestVersion,
				})
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
	return found
}
