# fzt-terminal

Ecosystem repo for fzt interactive tools. Provides everything that makes fzt visual and interactive: terminal and browser renderers, the command palette, style system, and frontend behavior. Imports the fzt engine (`github.com/nelsong6/fzt`) for state management, scoring, and tree logic.

## Architecture

```
fzt (engine)                    fzt-terminal (this repo)
  core.State, core.HandleKey      command.go      -- command palette, identity, action routing
  core.TreeContext, scoring        tui/            -- terminal renderer (tcell + raw TTY)
  render.Canvas, render.Session   web/            -- browser renderer (JS + CSS)
  core.LoadYAML                   cmd/wasm/       -- WASM bridge (Go -> JS)
                                  cmd/automate/   -- shell automation binary
                                  build/          -- build script with version injection
```

## Package guide

### `terminal` (root package, `command.go`)

Shared frontend behavior imported by every fzt app:

- **`InjectCommandFolder`** -- appends the hidden `:` command folder to the item tree. Single-level (`:` -> core commands) when no frontend is registered; two-level (`:` -> frontend commands + `::` -> core commands) when `FrontendName` is set.
- **`HandleCommandAction`** -- routes leaf selections in the command tree. Version "on" reads a registry index from Fields[2] to look up the version string from `State.VersionRegistry`. "off" clears `VersionDisplay`. Frontend commands matched by name → action string.
- **`EngineVersion`** -- module-level var set via ldflags. Used in the version registry for the `::` core level.
- **`IsInCommandScope` / `ScopeCtlTitle`** -- scope awareness for renderers (show "fzt ctl" vs "<frontend> ctl" in the title bar).
- **`ApplyConfig`** -- sets frontend identity (name, version, commands) from Config onto State before command injection.

### `tui/` -- Terminal renderer

Full-screen and inline TUI rendering via tcell. Key files:

| File | Purpose |
|------|---------|
| `tui.go` | Entry points: `Run` (full-screen), `RunInline` (inline), `NewSession`/`NewTreeSession` (headless), `Simulate` (testing). Layout functions: `drawDefault`, `drawReverse`, `drawUnified` (tree mode). |
| `tree.go` | Unified tree renderer: `drawUnified` draws prompt bar + tree as a single navigation surface. `drawTreeRow` renders individual rows with icons, indentation, match highlighting. |
| `style.go` | Semantic color constants (Catppuccin Mocha palette mapped to tcell colors). CSS equivalents documented in comments. |
| `canvas.go` | `tcellCanvas` wraps `tcell.Screen` to satisfy `render.Canvas`. |
| `inline.go` | `RunInline` -- renders TUI inline in the terminal buffer (no alternate screen) using raw ANSI escape sequences + `MemScreen`. Used when `--height` is specified. |
| `keyparse.go` | Raw byte -> tcell key parser for inline mode. Handles CSI/SS3 escape sequences, UTF-8, control characters. |
| `rawreader.go` | `rawTerminal` abstraction for raw TTY I/O. |
| `rawreader_unix.go` | Unix: opens `/dev/tty`, `term.MakeRaw`. |
| `rawreader_windows.go` | Windows: opens `CONIN$/CONOUT$`, enables VT processing + VT input. |
| `commands.go` | Currently empty (placeholder). |

### `web/` -- Browser renderer

JavaScript/CSS for rendering fzt in the browser via WASM:

| File | Purpose |
|------|---------|
| `fzt-terminal.js` | Core browser terminal. ANSI parser, grid renderer (styled HTML spans), font metrics, keyboard forwarding, resize observer. Factory: `createFztTerminal(container, options)`. |
| `fzt-web.js` | Higher-level wrapper with Catppuccin Mocha defaults (palette, Perfect DOS VGA 437 font, Nerd Font icons). Factory: `createFztWeb(container, options)`. |
| `fzt-dom-renderer.js` | Alternative DOM renderer using structured data API (`getVisibleRows`, `getPromptState`, `getUIState`) instead of ANSI parsing. Renders native DOM elements with CSS classes. |
| `fzt-terminal.css` | CRT terminal styles: scanline overlay, vignette, cursor blink, custom properties (`--fzt-*`). |

### `cmd/wasm/` -- WASM bridge

Compiles to `fzt.wasm`. Exposes a global `fzt` object to JavaScript:

- `fzt.loadYAML(yaml)` -- parse YAML items
- `fzt.setFrontend({name, version})` -- register frontend identity
- `fzt.addCommands([{name, description, action}])` -- register frontend commands for `:` palette
- `fzt.init(cols, rows)` -- create session, returns `{ansi, cursorX, cursorY}`
- `fzt.handleKey(key, ctrl, shift)` -- process keyboard event, returns frame + action
- `fzt.clickRow(row)` -- process mouse click on visual row
- `fzt.resize(cols, rows)` -- resize terminal
- `fzt.getVisibleRows()` / `fzt.getPromptState()` / `fzt.getUIState()` -- structured data API for DOM renderer

### `cmd/automate/` -- Shell automation binary

Standalone CLI tool: loads a YAML menu, presents an interactive tree picker via `tui.Run`, prints selected leaf to stdout. Shell wrappers execute the selection as a function.

Usage: `fzt-automate --yaml <path> [--title "..."] [--header "Name\tDescription"]`

### `build/` -- Build script

`go run ./build` builds the native automate binary with version injection via ldflags (`-X github.com/nelsong6/fzt/render.Version=<git describe>`). `go run ./build wasm` builds the WASM binary.

## Style system

- **Palette**: Catppuccin Mocha. Terminal uses tcell color constants mapped to the 16-color palette. Browser uses the same hex values via `fzt-web.js` DEFAULT_PALETTE and CSS custom properties.
- **Font**: Perfect DOS VGA 437 (primary), Cascadia Code / Fira Code / JetBrains Mono (fallbacks). Nerd Font Mono for icons.
- **CRT effects** (browser only): scanline overlay, vignette, border-radius, font-smoothing disabled.

## Command palette

The `:` item is a hidden folder injected into every fzt app's item tree.

- **No frontend registered** (`FrontendName == ""`): `:` -> core commands (version on/off, update).
- **Frontend registered**: `:` -> frontend commands + `::` subfolder -> core commands. The frontend registers commands via `State.FrontendCommands` or WASM `fzt.addCommands()`.

The prompt shows scope breadcrumbs when inside command folders. `ScopeCtlTitle` returns "fzt ctl" at `::` depth or "<frontend> ctl" at `:` depth.

## Dependencies

- `github.com/nelsong6/fzt v0.1.30` -- engine (state, scoring, tree logic, YAML parsing, render abstractions)
- `github.com/gdamore/tcell/v2` -- terminal screen library
- `golang.org/x/term` -- raw terminal mode
- `golang.org/x/sys` -- Windows console API

## Building

```bash
# Automate binary (native)
go run ./build
# or directly:
go build -o fzt-automate ./cmd/automate

# WASM binary
go run ./build wasm
# or directly:
GOOS=js GOARCH=wasm go build -o fzt.wasm ./cmd/wasm
```

The build script injects version from `git describe --tags --always --dirty` via ldflags.

## CI

GitHub Actions workflow (`.github/workflows/build.yml`) on push to main:

1. **Build matrix**: WASM (`fzt.wasm`), automate binaries (windows-amd64, linux-amd64, darwin-arm64)
2. **Release**: auto-increments patch version, creates GitHub release with all binaries + web assets (`fzt-terminal.js`, `fzt-terminal.css`, `fzt-web.js`, `fzt-dom-renderer.js`)
3. **Downstream dispatch**: notifies `fzt-showcase` and `my-homepage` repos via `repository_dispatch` (uses GitHub App token from Azure Key Vault for cross-repo auth)

## Key patterns

- **Headless sessions**: `tui.NewSession` / `tui.NewTreeSession` create sessions that render to `MemScreen` without a real terminal. Used by WASM bridge and `Simulate`.
- **Dual render paths**: ANSI-based (terminal + `fzt-terminal.js`) and structured data (DOM renderer via `getVisibleRows` etc.).
- **Platform abstraction**: `rawreader_unix.go` / `rawreader_windows.go` handle TTY differences for inline mode.
- **Type aliases**: `tui.Config`, `tui.Session`, `tui.SessionFrame` alias engine types so callers compile after refactors.
