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
	ldflags := fmt.Sprintf("-X github.com/nelsong6/fzt/render.Version=%s", version)

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
