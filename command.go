// Package terminal provides shared frontend behavior for fzt tools:
// command palette mechanics, frontend identity, and action routing.
//
// Every tool that wants to be an "fzt app" imports this package alongside
// the fzt engine (github.com/nelsong6/fzt/core).
package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	if coreVerStr == "" || coreVerStr == "dev" {
		coreVerStr = "ERROR: use go run ./build"
	}

	// Build version registry — each entry gets an index that "on" buttons reference
	s.VersionRegistry = nil
	if hasFrontend {
		feLabel := s.FrontendName
		feVer := s.FrontendVersion
		if feVer == "" || feVer == "UNSET" {
			feVer = "ERROR: use go run ./build"
		}
		s.VersionRegistry = append(s.VersionRegistry, feLabel+" "+feVer) // index 0: frontend
		s.VersionRegistry = append(s.VersionRegistry, "fzt "+coreVerStr) // index 1: engine
		identity := s.IdentityLabel
		if identity == "" {
			identity = "(none)"
		}
		s.VersionRegistry = append(s.VersionRegistry, identity) // index 2: identity
	} else {
		s.VersionRegistry = append(s.VersionRegistry, "fzt "+coreVerStr) // index 0: engine
	}

	base := len(ctx.AllItems)
	ctlFolderIdx := base
	var items []core.Item

	if hasFrontend {
		items = buildTwoLevelCommandTree(s, ctlFolderIdx, 0, 1) // feIdx=0, coreIdx=1
	} else {
		items = buildCoreLevelCommandTree(s.VersionRegistry, ctlFolderIdx, 0) // coreIdx=0
	}

	ctx.AllItems = append(ctx.AllItems, items...)
	ctx.Items = core.RootItemsOf(ctx.AllItems)
}

// buildCoreLevelCommandTree builds `:` → core commands directly (no frontend layer).
// versionIdx is the index into State.VersionRegistry for this level's version string.
func buildCoreLevelCommandTree(registry []string, ctlFolderIdx int, versionIdx int) []core.Item {
	idx := ctlFolderIdx + 1

	versionItemIdx := idx
	idx++
	updateIdx := idx
	idx++
	updatetimerIdx := idx
	idx++
	validateIdx := idx

	ctlChildren := []int{versionItemIdx, updateIdx, updatetimerIdx, validateIdx}

	versionDesc := ""
	if versionIdx >= 0 && versionIdx < len(registry) {
		versionDesc = registry[versionIdx]
	}

	return []core.Item{
		{
			Fields: []string{":"}, Depth: 0, HasChildren: true,
			ParentIdx: -1, Children: ctlChildren, Hidden: true,
		},
		{
			Fields: []string{"version", versionDesc}, Depth: 1,
			ParentIdx: ctlFolderIdx,
		},
		{Fields: []string{"update", "Update fzt to latest release"}, Depth: 1, ParentIdx: ctlFolderIdx},
		{Fields: []string{"updatetimer", "Show time to next sync check"}, Depth: 1, ParentIdx: ctlFolderIdx},
		{Fields: []string{"validate", "Validate credential store"}, Depth: 1, ParentIdx: ctlFolderIdx},
	}
}

// buildTwoLevelCommandTree builds `:` → frontend commands + `::` → core commands.
// feIdx and coreIdx are indices into State.VersionRegistry.
//
// Index allocation: the function pre-allocates contiguous index ranges for all items
// before building the slice. Starting from ctlFolderIdx+1, it reserves indices for:
// version (1), whoami folder (3), each FrontendCommand + its Children,
// and the core subfolder. Items must be appended in the same order as indices were reserved.
func buildTwoLevelCommandTree(s *core.State, ctlFolderIdx int, feIdx int, coreIdx int) []core.Item {
	idx := ctlFolderIdx + 1
	var ctlChildren []int

	// frontend version — single toggle leaf
	feVersionIdx := idx
	ctlChildren = append(ctlChildren, feVersionIdx)
	idx++

	// whoami folder
	identityFolderIdx := idx
	ctlChildren = append(ctlChildren, identityFolderIdx)
	idx++
	identityOnIdx := idx
	idx++
	identityOffIdx := idx
	idx++

	for _, cmd := range s.FrontendCommands {
		ctlChildren = append(ctlChildren, idx)
		idx++
		idx += len(cmd.Children) // reserve indices for children
	}

	coreSubfolderIdx := idx
	ctlChildren = append(ctlChildren, coreSubfolderIdx)
	idx++

	// core version — single toggle leaf
	coreVersionIdx := idx
	coreSubChildren := []int{coreVersionIdx}
	idx++

	coreUpdateIdx := idx
	coreSubChildren = append(coreSubChildren, coreUpdateIdx)
	idx++

	coreUpdatetimerIdx := idx
	coreSubChildren = append(coreSubChildren, coreUpdatetimerIdx)
	idx++

	coreValidateIdx := idx
	coreSubChildren = append(coreSubChildren, coreValidateIdx)

	var items []core.Item

	items = append(items, core.Item{
		Fields: []string{":"}, Depth: 0, HasChildren: true,
		ParentIdx: -1, Children: ctlChildren, Hidden: true,
	})

	// Frontend version toggle — description shows the version string
	feVersionDesc := ""
	if feIdx >= 0 && feIdx < len(s.VersionRegistry) {
		feVersionDesc = s.VersionRegistry[feIdx]
	}
	items = append(items, core.Item{
		Fields: []string{"version", feVersionDesc}, Depth: 1,
		ParentIdx: ctlFolderIdx,
	})

	identityIdxStr := fmt.Sprintf("%d", len(s.VersionRegistry)-1)
	items = append(items, core.Item{
		Fields: []string{"whoami", "Show/hide loaded identity"}, Depth: 1,
		HasChildren: true, ParentIdx: ctlFolderIdx,
		Children: []int{identityOnIdx, identityOffIdx},
	})
	items = append(items, core.Item{Fields: []string{"on", "Show identity", identityIdxStr}, Depth: 2, ParentIdx: identityFolderIdx})
	items = append(items, core.Item{Fields: []string{"off", "Hide identity"}, Depth: 2, ParentIdx: identityFolderIdx})

	for _, cmd := range s.FrontendCommands {
		cmdIdx := ctlFolderIdx + len(items)
		hasChildren := len(cmd.Children) > 0
		cmdItem := core.Item{
			Fields: []string{cmd.Name, cmd.Description}, Depth: 1,
			ParentIdx: ctlFolderIdx, HasChildren: hasChildren,
		}
		if hasChildren {
			for i := range cmd.Children {
				cmdItem.Children = append(cmdItem.Children, cmdIdx+1+i)
			}
		}
		items = append(items, cmdItem)
		for _, child := range cmd.Children {
			items = append(items, core.Item{
				Fields: []string{child.Name, child.Description}, Depth: 2, ParentIdx: cmdIdx,
			})
		}
	}

	items = append(items, core.Item{
		Fields: []string{":", "fzt core"}, Depth: 1,
		HasChildren: true, ParentIdx: ctlFolderIdx, Children: coreSubChildren,
	})

	// Core version toggle — description shows the engine version string
	coreVersionDesc := ""
	if coreIdx >= 0 && coreIdx < len(s.VersionRegistry) {
		coreVersionDesc = s.VersionRegistry[coreIdx]
	}
	items = append(items, core.Item{
		Fields: []string{"version", coreVersionDesc}, Depth: 2,
		ParentIdx: coreSubfolderIdx,
	})

	items = append(items, core.Item{Fields: []string{"update", "Update fzt to latest release"}, Depth: 2, ParentIdx: coreSubfolderIdx})
	items = append(items, core.Item{Fields: []string{"updatetimer", "Show time to next sync check"}, Depth: 2, ParentIdx: coreSubfolderIdx})
	items = append(items, core.Item{Fields: []string{"validate", "Validate credential store"}, Depth: 2, ParentIdx: coreSubfolderIdx})

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
	case "version":
		// Toggle: if title already shows this version, clear it; otherwise set it
		if len(item.Fields) >= 2 && item.Fields[1] != "" {
			ver := item.Fields[1]
			if s.TitleOverride == ver {
				s.ClearTitle()
			} else {
				s.SetTitle(ver, 1)
			}
		}
		return ""
	case "on":
		// "on" items store their VersionRegistry index in Fields[2] (used by whoami).
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
	case "updatetimer":
		if s.SyncTimerShown {
			s.ClearTitle()
		} else {
			s.SyncTimerShown = true
		}
		return ""
	case "validate":
		HandleValidate(s)
		return ""
	case "load-nelson", "load-nelson-ea", "load-nelson-r1":
		identity := strings.TrimPrefix(name, "load-")
		if s.ConfigDir == "" {
			s.SetTitle("no config directory set", 2)
			return ""
		}
		if err := os.WriteFile(filepath.Join(s.ConfigDir, ".identity"), []byte(identity), 0644); err != nil {
			s.SetTitle(fmt.Sprintf("failed to write identity: %v", err), 2)
			return ""
		}
		s.IdentityLabel = identity
		// Auto-sync after loading identity
		secret := s.JWTSecret
		if secret == "" {
			var err error
			secret, err = ReadJWTSecret()
			if err != nil {
				s.SetTitle(err.Error(), 2)
				return ""
			}
			s.JWTSecret = secret
		}
		if _, err := SyncMenu(s.ConfigDir, secret); err != nil {
			s.SetTitle(fmt.Sprintf("loaded %s but sync failed: %v", identity, err), 2)
			return ""
		}
		return "loaded"
	case "unload":
		if s.ConfigDir != "" {
			os.Remove(filepath.Join(s.ConfigDir, ".identity"))
			os.Remove(filepath.Join(s.ConfigDir, "menu-cache.yaml"))
		}
		return "unloaded"
	case "sync":
		if s.ConfigDir == "" {
			s.SetTitle("no config directory set", 2)
			return ""
		}
		secret := s.JWTSecret
		if secret == "" {
			var err error
			secret, err = ReadJWTSecret()
			if err != nil {
				s.SetTitle(err.Error(), 2)
				return ""
			}
			s.JWTSecret = secret
		}
		count, err := SyncMenu(s.ConfigDir, secret)
		if err != nil {
			s.SetTitle(err.Error(), 2)
			return ""
		}
		identityName, _ := ReadTrimmedFile(filepath.Join(s.ConfigDir, ".identity"))
		s.SetTitle(fmt.Sprintf("synced %d items for %s", count, identityName), 1)
		return ""
	case "update":
		return "update"
	}

	for _, cmd := range s.FrontendCommands {
		if cmd.Name == name {
			return cmd.Action
		}
		for _, child := range cmd.Children {
			if child.Name == name {
				return child.Action
			}
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
	if cfg.InitialDisplay != "" {
		s.IdentityLabel = cfg.InitialDisplay
	}
}
