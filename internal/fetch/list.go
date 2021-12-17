package fetch

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/anorth/godep/pkg/model"
)

// Lists all active modules under a path. The main module is the one contained in modulePath, and the active
// modules are the main module and its dependencies.
func ListModules(modulePath string) ([]*model.ModuleInfo, error) {
	raw, err := Exec(modulePath, "go", "list", "-json", "-m", "all")
	if err != nil {
		return nil, fmt.Errorf("failed listing packages for %s: %w", modulePath, err)
	}

	raw = fixListJson(raw)
	var moduleList []*model.ModuleInfo
	if err = json.Unmarshal(raw, &moduleList); err != nil {
		return nil, fmt.Errorf("failed parsing module information: %w", err)
	}

	return moduleList, nil
}

// Lists all packages transitively depended upon by a path.
func ListPackages(packagePath string) ([]*model.PackageInfo, error) {
	raw, err := Exec(packagePath, "go", "list", "-json", "all")
	if err != nil {
		return nil, fmt.Errorf("failed listing packages for %s: %w", packagePath, err)
	}

	// The output is a sequence of PackageInfo structs, lacking delimiting commas or
	// surrounding brackets to form an array.
	raw = fixListJson(raw)
	var packageList []*model.PackageInfo
	if err = json.Unmarshal(raw, &packageList); err != nil {
		return nil, fmt.Errorf("failed parsing package information: %w", err)
	}

	return packageList, nil
}

// The output from "go list" is a sequence of structs as JSON but lacking delimiting commas or
// surrounding brackets to form an array.
func fixListJson(raw []byte) []byte {
	raw = bytes.ReplaceAll(bytes.TrimSpace(raw), []byte("\n}\n"), []byte("\n},\n"))
	raw = append([]byte("[\n"), raw...)
	raw = append(raw, []byte("\n]")...)
	return raw
}

