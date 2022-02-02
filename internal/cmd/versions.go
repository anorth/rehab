package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/anorth/rehab/internal/db"
	"github.com/anorth/rehab/pkg/model"
)

type StaleVersion struct {
	Consumer        model.ModuleVersion // The module version requiring an old upstream dependency
	Requirement     model.ModuleVersion // The upstream module required and declared version
	SelectedVersion string              // The requirement version selected by MVS
	SelectedReason  model.ModuleVersion // A (transitively) required module that declares the MVS-selected version requirement
	TransitiveStale bool                // True if the requirement has transitively stale requirements and is not a latest version
	HighestVersion  string              // The highest available version of the requirement
}

func (sv *StaleVersion) String() string {
	via := fmt.Sprintf(" via %s", sv.SelectedReason)
	if sv.SelectedReason.Path == "" || sv.SelectedReason == sv.Consumer {
		via = ""
	}
	if sv.TransitiveStale {
		via = via + " (has stale transitive requirements)"
	}

	return fmt.Sprintf("%s requires %s, builds with %s%s (highest %s)",
		sv.Consumer, sv.Requirement, sv.SelectedVersion, via, sv.HighestVersion)
}

// Finds imports in a dependency tree based at some root module where a declared dependency module version
// does not match the module version chosen by MVS when building the root module.
// This situation means that the tests for the consuming module run with a different version of the
// dependency than that actually used in production.
func FindStaleVersions(modules *db.Modules, modGraph *db.ModGraph) []*StaleVersion {
	var found []*StaleVersion
	q := []model.ModuleVersion{{modules.Main().Path, modules.Main().Version}}
	// Records the modules in the graph which have been traversed already.
	modulesSeen := map[string]struct{}{q[0].Path: {}}
	// Records the stale relationships already recorded.
	type staleversionkey struct {
		consumer, requirement string
	}
	edgesSeen := map[staleversionkey]struct{}{}
	var node model.ModuleVersion
	for len(q) > 0 {
		node, q = q[0], q[1:]
		requirements := modGraph.UpstreamOf(node.Path, node.Version)
		downstreamInfo, err := modules.ForPath(node.Path)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "failed loading module of", node.Path, err)
			continue
		}
		downstreamIsOutdated := false
		for _, req := range requirements {
			key := staleversionkey{consumer: req.Downstream.Path, requirement: req.Upstream.Path}
			if _, ok := edgesSeen[key]; ok {
				continue
			}
			upstreamSelected, reason, err := modGraph.SelectedVersion(req.Upstream.Path)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "failed checking highest version of", req.Upstream.Path, err)
				continue
			}
			upstreamInfo, err := modules.ForPath(req.Upstream.Path)
			if err != nil {
				_, _ = fmt.Fprintln(os.Stderr, "failed loading module of", req.Upstream.Path, err)
				continue
			}
			upstreamLatest := upstreamInfo.Version
			if upstreamInfo.Update != nil {
				upstreamLatest = upstreamInfo.Update.Version
			}
			upstreamMismatch := req.Upstream.Version != upstreamSelected

			if upstreamMismatch {
				// Suggest updating the downstream module's mismatched requirement on the upstream.
				// If downstream is out of date, we can't check what version of the upstream would have been
				// selected by the most recent downstream version, but suggest upgrading anyway.
				// This suggestion can be ignored if it turns out to be irrelevant.
				found = append(found, &StaleVersion{
					Consumer:        req.Downstream,
					Requirement:     req.Upstream,
					SelectedVersion: upstreamSelected,
					SelectedReason:  reason,
					TransitiveStale: false,
					HighestVersion:  upstreamLatest,
				})
				edgesSeen[key] = struct{}{}

				// If downstream is out of date but was pulling in an old version of some requirement,
				// suggest updating the consumers of this out-of-date downstream module, too.
				if downstreamInfo.Update != nil {
					downstreamIsOutdated = true
				}
			} else {
				// Trace through deeper in the requirement graph only for the version of the upstream
				// that is the one selected by MVS.
				_, seen := modulesSeen[req.Upstream.Path]
				if !seen && (! strings.HasPrefix(req.Upstream.Path, "golang.org/")) { // Replace with whitelist?
					q = append(q, req.Upstream)
					modulesSeen[req.Upstream.Path] = struct{}{}
				}
			}
		}

		if downstreamIsOutdated {
			downstreamLatest := downstreamInfo.Update
			requirers := modGraph.DownstreamOf(downstreamInfo.Path, downstreamInfo.Version)
			for _, req := range requirers {
				key := staleversionkey{consumer: req.Downstream.Path, requirement: req.Upstream.Path}
				if _, ok := edgesSeen[key]; ok {
					continue
				}
				found = append(found, &StaleVersion{
					Consumer:        req.Downstream,
					Requirement:     req.Upstream,
					SelectedVersion: downstreamInfo.Version,
					SelectedReason:  req.Downstream,
					TransitiveStale: true,
					HighestVersion:  downstreamLatest.Version,
				})
				edgesSeen[key] = struct{}{}
			}
		}
	}
	return found
}
