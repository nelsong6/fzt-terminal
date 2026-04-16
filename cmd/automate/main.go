// fzt-automate — shell automation tool powered by fzt.
//
// Loads a YAML menu, presents an interactive tree picker, and prints the
// selected leaf name to stdout. The shell wrapper executes it as a function.
//
// Usage:
//
//	fzt-automate --yaml /path/to/menu.yaml
//	fzt-automate --yaml /path/to/menu.yaml --title "What would you like to do?"
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nelsong6/fzt/core"
	"github.com/nelsong6/fzt/render"
	"github.com/nelsong6/fzt-terminal/tui"
)

func main() {
	if render.Version == "UNSET" {
		fmt.Fprintln(os.Stderr, "fzt-automate: version not set — use 'go run ./build automate' or build with ldflags")
		os.Exit(1)
	}

	yamlPath := ""
	title := "What would you like to do?"
	header := "Name\tDescription"

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--yaml":
			if i+1 < len(args) {
				yamlPath = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		case "--header":
			if i+1 < len(args) {
				header = args[i+1]
				i++
			}
		case "--version":
			fmt.Println(render.Version)
			os.Exit(0)
		}
	}

	if yamlPath == "" {
		fmt.Fprintln(os.Stderr, "fzt-automate: --yaml is required")
		os.Exit(1)
	}

	configDir := filepath.Dir(yamlPath)
	cacheFile := filepath.Join(configDir, "menu-cache.yaml")

	var items []core.Item
	if _, err := os.Stat(cacheFile); err == nil {
		items, err = core.LoadYAML(cacheFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fzt-automate: %v\n", err)
			os.Exit(1)
		}
	}

	// Read loaded identity for whoami display
	identity := ""
	identityFile := filepath.Join(configDir, ".identity")
	if data, err := os.ReadFile(identityFile); err == nil {
		identity = strings.TrimSpace(string(data))
	}

	// Read persisted menu version for conflict detection on save
	menuVersion := 0
	versionFile := filepath.Join(configDir, ".menu-version")
	if data, err := os.ReadFile(versionFile); err == nil {
		menuVersion, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}

	if header != "" {
		headerFields := strings.Split(header, "\t")
		headerItem := core.Item{Fields: headerFields, Depth: -1}
		items = append([]core.Item{headerItem}, items...)
	}

	cfg := tui.Config{
		Layout:          "reverse",
		Border:          true,
		Tiered:          true,
		DepthPenalty:    5,
		HeaderLines:     1,
		Nth:             []int{1},
		AcceptNth:       []int{1},
		Title:           title,
		TreeMode:        true,
		EnvTags:         []string{"terminal"},
		FrontendName:    "automate",
		FrontendVersion: render.Version,
		InitialDisplay:  identity,
		ConfigDir:          configDir,
		InitialMenuVersion: menuVersion,
		FrontendCommands: []core.CommandItem{
			{Name: "load", Description: "Load an identity profile", Children: []core.CommandItem{
				{Name: "load-nelson", Description: "Personal account", Action: "load-nelson"},
				{Name: "load-nelson-ea", Description: "Engineered Arts", Action: "load-nelson-ea"},
				{Name: "load-nelson-r1", Description: "R1", Action: "load-nelson-r1"},
			}},
			{Name: "unload", Description: "Clear loaded identity", Action: "unload"},
			{Name: "sync", Description: "Sync menu from cloud", Action: "sync"},
			{Name: "edit", Description: "Edit menu tree", Children: []core.CommandItem{
				{Name: "add-after", Description: "Add item after cursor", Action: "add-after"},
				{Name: "add-folder", Description: "Create folder at cursor", Action: "add-folder"},
				{Name: "edit-item", Description: "Edit item properties", Action: "rename"},
				{Name: "delete", Description: "Delete highlighted item", Action: "delete"},
				{Name: "save", Description: "Save changes to cloud", Action: "save"},
			}},
			{Name: "shortcuts", Description: "Keyboard shortcuts", Children: []core.CommandItem{
				{Name: "shift", Description: "modifier key (all shortcuts)"},
				{Name: "shift+enter", Description: "confirm action"},
				{Name: "shift+back", Description: "return to home"},
				{Name: "S", Description: "sync menu from cloud"},
				{Name: "W", Description: "save changes to cloud"},
				{Name: "A", Description: "add item after cursor"},
				{Name: "F", Description: "create folder at cursor"},
				{Name: "R", Description: "edit item properties"},
				{Name: "I", Description: "edit item properties"},
				{Name: "D", Description: "delete highlighted item"},
				{Name: "H", Description: "navigate left (vim)"},
				{Name: "J", Description: "navigate down (vim)"},
				{Name: "K", Description: "navigate up (vim)"},
				{Name: "L", Description: "navigate right (vim)"},
			}},
		},
	}

	result, err := tui.Run(items, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fzt-automate: %v\n", err)
		os.Exit(1)
	}

	if result == "" {
		os.Exit(130)
	}

	if result == "unloaded" {
		fmt.Fprintln(os.Stderr, "identity unloaded")
		os.Exit(130)
	}

	if result == "loaded" || result == "synced" {
		fmt.Fprintln(os.Stderr, "synced — reopen to see menu")
		os.Exit(130)
	}

	// Look up the selected item to check for action overrides.
	// URL action = bookmark (open in browser). Command action = stable identifier (survives renames).
	for _, item := range items {
		if len(item.Fields) > 0 && item.Fields[0] == result {
			if item.Action != nil {
				fmt.Println(item.Action.Target)
				os.Exit(0)
			}
			break
		}
	}

	fmt.Println(result)
}
