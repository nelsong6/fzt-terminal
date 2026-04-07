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
	"strings"

	"github.com/nelsong6/fzt/core"
	"github.com/nelsong6/fzt-terminal/tui"
)

var Version = "dev"

func main() {
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
			fmt.Println(Version)
			os.Exit(0)
		}
	}

	if yamlPath == "" {
		fmt.Fprintln(os.Stderr, "fzt-automate: --yaml is required")
		os.Exit(1)
	}

	items, err := core.LoadYAML(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fzt-automate: %v\n", err)
		os.Exit(1)
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
		FrontendName:    "automate",
		FrontendVersion: Version,
	}

	result, err := tui.Run(items, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fzt-automate: %v\n", err)
		os.Exit(1)
	}

	if result == "" {
		os.Exit(130)
	}

	fmt.Println(result)
}
