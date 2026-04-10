// Build script for fzt. Use "go run ./build" instead of "go build".
// Injects git describe version via ldflags so both native and WASM
// binaries carry the correct version without manual flags.
//
// Usage:
//
//	go run ./build              # native binary (fzt.exe on Windows)
//	go run ./build wasm         # WASM binary (fzt.wasm)
//	go run ./build automate     # fzt-automate binary
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func main() {
	version := "dev"
	out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
	if err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			version = v
		}
	}

	target := ""
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	ext := ""
	pkg := "."
	output := "fzt"
	env := os.Environ()

	switch target {
	case "wasm":
		ext = ".wasm"
		pkg = "./cmd/wasm"
		env = append(env, "GOOS=js", "GOARCH=wasm")
	case "automate":
		pkg = "./cmd/automate"
		output = "fzt-automate"
		if runtime.GOOS == "windows" {
			ext = ".exe"
		}
	default:
		if runtime.GOOS == "windows" {
			ext = ".exe"
		}
	}

	output += ext

	// Read fzt engine version from go.mod (or replace directive)
	engineVersion := "dev"
	modList, err := exec.Command("go", "list", "-m", "-json", "github.com/nelsong6/fzt").Output()
	if err == nil {
		modStr := string(modList)
		if strings.Contains(modStr, `"Replace"`) {
			// Local replace — git describe in the replace dir
			dirIdx := strings.Index(modStr, `"Replace"`)
			if dirIdx >= 0 {
				sub := modStr[dirIdx:]
				if di := strings.Index(sub, `"Dir": "`); di >= 0 {
					start := di + 8
					end := strings.Index(sub[start:], `"`)
					if end >= 0 {
						dir := sub[start : start+end]
						gitOut, gitErr := exec.Command("git", "-C", dir, "describe", "--tags", "--always", "--dirty").Output()
						if gitErr == nil {
							if v := strings.TrimSpace(string(gitOut)); v != "" {
								engineVersion = v
							}
						}
					}
				}
			}
		} else if vi := strings.Index(modStr, `"Version": "`); vi >= 0 {
			// Normal module — use version from go.mod
			start := vi + 12
			end := strings.Index(modStr[start:], `"`)
			if end >= 0 {
				engineVersion = modStr[start : start+end]
			}
		}
	}

	ldflags := fmt.Sprintf("-X github.com/nelsong6/fzt/render.Version=%s -X github.com/nelsong6/fzt-terminal.EngineVersion=%s", version, engineVersion)

	args := []string{"build", "-ldflags", ldflags, "-o", output, pkg}
	fmt.Printf("go %s\n", strings.Join(args, " "))

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
