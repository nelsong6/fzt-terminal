// Package terminal provides shared frontend behavior for fzt tools:
// command palette mechanics, frontend identity, and action routing.
//
// Every tool that wants to be an "fzt app" imports this package alongside
// the fzt engine (github.com/nelsong6/fzt/core).
package terminal

import (
	"fmt"
	"strconv"

	"github.com/nelsong6/fzt/core"
)

// EngineVersion is the fzt engine version this build was compiled against.
// Set via ldflags: -X github.com/nelsong6/fzt-terminal.EngineVersion=v0.2.39
var EngineVersion = "dev"

// InjectCommandFolder appends the `:` command folder and its children to the
// tree's AllItems. When a frontend is registered (FrontendName set), the first
// level holds frontend commands and a nested `:` subfolder holds core commands.
// When no frontend is registered, the first level holds core commands directly.
func InjectCommandFolder(s *core.State, coreVersion string) {
	ctx := s.TopCtx()
	hasFrontend := s.FrontendName != ""

	coreVerStr := coreVersion
	if coreVerStr == "" {
		coreVerStr = "dev"
	}

	// Build version registry — each entry gets an index that "on" buttons reference
	s.VersionRegistry = nil
	if hasFrontend {
		feLabel := s.FrontendName
		feVer := s.FrontendVersion
		if feVer == "" {
			feVer = "unknown"
		}
		s.VersionRegistry = append(s.VersionRegistry, feLabel+" "+feVer) // index 0: frontend
		s.VersionRegistry = append(s.VersionRegistry, "fzt "+coreVerStr) // index 1: engine
	} else {
		s.VersionRegistry = append(s.VersionRegistry, "fzt "+coreVerStr) // index 0: engine
	}

	base := len(ctx.AllItems)
	ctlFolderIdx := base
	var items []core.Item

	if hasFrontend {
		items = buildTwoLevelCommandTree(s, ctlFolderIdx, 0, 1) // feIdx=0, coreIdx=1
	} else {
		items = buildCoreLevelCommandTree(ctlFolderIdx, 0) // coreIdx=0
	}

	ctx.AllItems = append(ctx.AllItems, items...)
	ctx.Items = core.RootItemsOf(ctx.AllItems)
}

// buildCoreLevelCommandTree builds `:` → core commands directly (no frontend layer).
// versionIdx is the index into State.VersionRegistry for this level's version string.
func buildCoreLevelCommandTree(ctlFolderIdx int, versionIdx int) []core.Item {
	idx := ctlFolderIdx + 1

	versionFolderIdx := idx
	idx++
	versionOnIdx := idx
	idx++
	versionOffIdx := idx
	idx++
	updateIdx := idx

	ctlChildren := []int{versionFolderIdx, updateIdx}
	idxStr := fmt.Sprintf("%d", versionIdx)

	return []core.Item{
		{
			Fields: []string{":"}, Depth: 0, HasChildren: true,
			ParentIdx: -1, Children: ctlChildren, Hidden: true,
		},
		{
			Fields: []string{"version", "Show/hide version in title bar"}, Depth: 1,
			HasChildren: true, ParentIdx: ctlFolderIdx,
			Children: []int{versionOnIdx, versionOffIdx},
		},
		{Fields: []string{"on", "Show version", idxStr}, Depth: 2, ParentIdx: versionFolderIdx},
		{Fields: []string{"off", "Hide version"}, Depth: 2, ParentIdx: versionFolderIdx},
		{Fields: []string{"update", "Update fzt to latest release"}, Depth: 1, ParentIdx: ctlFolderIdx},
	}
}

// buildTwoLevelCommandTree builds `:` → frontend commands + `::` → core commands.
// feIdx and coreIdx are indices into State.VersionRegistry.
func buildTwoLevelCommandTree(s *core.State, ctlFolderIdx int, feIdx int, coreIdx int) []core.Item {
	idx := ctlFolderIdx + 1
	var ctlChildren []int

	feIdxStr := fmt.Sprintf("%d", feIdx)
	coreIdxStr := fmt.Sprintf("%d", coreIdx)

	feVersionFolderIdx := idx
	ctlChildren = append(ctlChildren, feVersionFolderIdx)
	idx++
	feVersionOnIdx := idx
	idx++
	feVersionOffIdx := idx
	idx++

	for range s.FrontendCommands {
		ctlChildren = append(ctlChildren, idx)
		idx++
	}

	coreSubfolderIdx := idx
	ctlChildren = append(ctlChildren, coreSubfolderIdx)
	idx++

	coreVersionFolderIdx := idx
	coreSubChildren := []int{coreVersionFolderIdx}
	idx++
	coreVersionOnIdx := idx
	idx++
	coreVersionOffIdx := idx
	idx++

	coreUpdateIdx := idx
	coreSubChildren = append(coreSubChildren, coreUpdateIdx)

	var items []core.Item

	items = append(items, core.Item{
		Fields: []string{":"}, Depth: 0, HasChildren: true,
		ParentIdx: -1, Children: ctlChildren, Hidden: true,
	})

	items = append(items, core.Item{
		Fields: []string{"version", "Show/hide version in title bar"}, Depth: 1,
		HasChildren: true, ParentIdx: ctlFolderIdx,
		Children: []int{feVersionOnIdx, feVersionOffIdx},
	})
	items = append(items, core.Item{Fields: []string{"on", "Show version", feIdxStr}, Depth: 2, ParentIdx: feVersionFolderIdx})
	items = append(items, core.Item{Fields: []string{"off", "Hide version"}, Depth: 2, ParentIdx: feVersionFolderIdx})

	for _, cmd := range s.FrontendCommands {
		items = append(items, core.Item{
			Fields: []string{cmd.Name, cmd.Description}, Depth: 1, ParentIdx: ctlFolderIdx,
		})
	}

	items = append(items, core.Item{
		Fields: []string{":", "fzt core"}, Depth: 1,
		HasChildren: true, ParentIdx: ctlFolderIdx, Children: coreSubChildren,
	})

	items = append(items, core.Item{
		Fields: []string{"version", "Show/hide version in title bar"}, Depth: 2,
		HasChildren: true, ParentIdx: coreSubfolderIdx,
		Children: []int{coreVersionOnIdx, coreVersionOffIdx},
	})
	items = append(items, core.Item{Fields: []string{"on", "Show version", coreIdxStr}, Depth: 3, ParentIdx: coreVersionFolderIdx})
	items = append(items, core.Item{Fields: []string{"off", "Hide version"}, Depth: 3, ParentIdx: coreVersionFolderIdx})

	items = append(items, core.Item{Fields: []string{"update", "Update fzt to latest release"}, Depth: 2, ParentIdx: coreSubfolderIdx})

	return items
}

// HandleCommandAction processes a selected leaf item in the command tree.
// Returns an action string for the frontend, or "" if handled internally.
func HandleCommandAction(s *core.State, item core.Item) string {
	if len(item.Fields) == 0 {
		return ""
	}
	name := item.Fields[0]

	switch name {
	case "on":
		// Third field is the version registry index
		if len(item.Fields) >= 3 {
			idx, err := strconv.Atoi(item.Fields[2])
			if err == nil && idx >= 0 && idx < len(s.VersionRegistry) {
				s.VersionDisplay = s.VersionRegistry[idx]
			}
		}
		return ""
	case "off":
		s.VersionDisplay = ""
		return ""
	case "update":
		return "update"
	}

	for _, cmd := range s.FrontendCommands {
		if cmd.Name == name {
			return cmd.Action
		}
	}

	return ""
}

// IsInCommandScope returns true if the current scope is inside a `:` folder.
func IsInCommandScope(s *core.State) bool {
	ctx := s.TopCtx()
	for _, level := range ctx.Scope[1:] {
		if level.ParentIdx >= 0 && level.ParentIdx < len(ctx.AllItems) {
			if len(ctx.AllItems[level.ParentIdx].Fields) > 0 && ctx.AllItems[level.ParentIdx].Fields[0] == ":" {
				return true
			}
		}
	}
	return false
}

// ScopeCtlTitle returns the appropriate title for the current scope.
// Returns "" if not inside a `:` command folder, "fzt ctl" if inside `::`,
// or "<frontendName> ctl" if inside the first `:`.
func ScopeCtlTitle(s *core.State) string {
	ctx := s.TopCtx()
	colonDepth := 0
	for _, level := range ctx.Scope[1:] {
		if level.ParentIdx >= 0 && level.ParentIdx < len(ctx.AllItems) {
			if len(ctx.AllItems[level.ParentIdx].Fields) > 0 && ctx.AllItems[level.ParentIdx].Fields[0] == ":" {
				colonDepth++
			}
		}
	}
	if colonDepth == 0 {
		return ""
	}
	if colonDepth >= 2 {
		return "fzt ctl"
	}
	name := s.FrontendName
	if name == "" {
		name = "fzt"
	}
	return name + " ctl"
}

// ApplyConfig sets frontend identity and commands from Config onto State.
// Call before InjectCommandFolder.
func ApplyConfig(s *core.State, cfg core.Config) {
	if cfg.FrontendName != "" {
		s.FrontendName = cfg.FrontendName
	}
	if cfg.FrontendVersion != "" {
		s.FrontendVersion = cfg.FrontendVersion
	}
	if len(cfg.FrontendCommands) > 0 {
		s.FrontendCommands = cfg.FrontendCommands
	}
}
