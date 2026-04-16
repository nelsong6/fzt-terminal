# fzt-terminal

Ecosystem repo for fzt interactive tools. Provides everything that makes fzt visual and interactive: terminal and browser renderers, the command palette, style system, and frontend behavior. Imports the fzt engine (`github.com/nelsong6/fzt`) for state management, scoring, and tree logic.

## Architecture

```
fzt (engine)                    fzt-terminal (this repo)
  core.State, core.HandleKey      command.go      -- command palette, identity, action routing
  core.TreeContext, scoring        credential.go   -- credential store (go-keyring)
  render.Canvas, render.Session   sync.go         -- API sync, JWT minting, menu-cache.yaml
  core.LoadYAML                   tui/            -- terminal renderer (tcell + raw TTY)
                                  web/            -- browser renderer (JS + CSS)
                                  cmd/wasm/       -- WASM bridge (Go -> JS)
                                  cmd/automate/   -- shell automation binary
                                  build/          -- build script with version injection
```

## Package guide

### `terminal` (root package, `command.go`, `credential.go`, `sync.go`)

Shared frontend behavior imported by every fzt app:

- **`InjectCommandFolder`** -- appends the hidden `:` command folder to the item tree. Single-level (`:` -> core commands) when no frontend is registered; two-level (`:` -> frontend commands + `::` -> core commands) when `FrontendName` is set. Skips injection if palette already exists in loaded cache (data-driven mode).
- **`HandleCommandAction`** -- routes leaf selections in the command tree by `Item.Action.Target` (stable command identifier) with fallback to `Fields[0]`. Handles version toggle, validate, updatetimer, sync, edit modes (add-after, add-folder, rename, delete, inspect, save), and internalized shell commands (load-*, unload). Frontend commands matched by name or action string.
- **`EngineVersion`** -- module-level var injected via ldflags from the build script (reads fzt engine version from go.mod or git describe for local replace).
- **`IsInCommandScope` / `ScopeCtlTitle`** -- scope awareness for renderers (show "fzt ctl" vs "<frontend> ctl" in the title bar).
- **`ApplyConfig`** -- sets frontend identity (name, version, commands) from Config onto State before command injection.
- **EnvTags / DisplayCondition** -- environment-based command filtering. `Config.EnvTags` declares the runtime capabilities (e.g. `["terminal"]` for automate, `["wasm", "browser"]` for the WASM bridge). During command tree construction (`buildCoreLevelCommandTree` / `buildTwoLevelCommandTree`), items with a non-empty `DisplayCondition` are skipped unless the condition string is present in `EnvTags`. Example: the `update` command has `DisplayCondition: "terminal"` so it only appears in the terminal palette, never in the browser. Tags are defined in `core.Config`, propagated to `core.State.EnvTags`, and checked via `hasEnvTag()`. The engine's `core.Item.DisplayCondition` field and `core.State.EnvTags` field are the underlying storage.
- **`HandleValidate` / `ReadJWTSecret`** (`credential.go`) -- credential store integration via go-keyring (Windows Credential Manager / KWallet / macOS Keychain). HandleValidate is invoked from the validate command in the `::` core palette.
- **`sync.go`** (root package) -- API-backed menu sync. JWT minting, API fetching (`/api/menu`), YAML serialization to `menu-cache.yaml`. `SyncMenu` fetches and caches the menu (returns item count, version, error). `SaveMenu` PUTs the menu tree with conflict detection via `baseVersion` (409 on stale). Both persist the version number to `.menu-version` for next launch.

### `tui/` -- Terminal renderer

Full-screen and inline TUI rendering via tcell. Key files:

| File | Purpose |
|------|---------|
| `tui.go` | Entry points: `Run` (full-screen), `RunInline` (inline), `NewSession`/`NewTreeSession` (headless), `Simulate` (testing). Layout functions: `drawDefault`, `drawReverse`, `drawUnified` (tree mode). `drawBorderTopWithTitle` renders the title bar with titleStyleHint (0=default, 1=green, 2=red) and syncIcon parameters. |
| `tree.go` | Unified tree renderer: `drawUnified` draws prompt bar + tree as a single navigation surface. `drawTreeRow` renders individual rows with icons, indentation, match highlighting. Headers start at borderOffset+5 to match tree row layout (2 selection + 2 icon + 1 buffer). Dynamic effectiveNameCol computed from visible items. |
| `sync.go` | Background sync check: `initSyncCheck`, `checkBookmarkStaleness`. 1-second ticker goroutine posts `tcell.EventInterrupt` for live redraws. 20-minute sync interval with `.last-sync-check` file. SyncIcon rendering (yellow icon in top-right corner). |
| `style.go` | Semantic color constants (Catppuccin Mocha palette mapped to tcell colors). CSS equivalents documented in comments. Exports `PaletteRGB` (tcell color → RGB map), `ColorToRGB()` (any tcell color to RGB), `BaseBgRGB`/`TextFgRGB` (default bg/fg), `DefaultFontName`/`DefaultFontSize` (shared font config). Used by GDI renderers (picker) to stay aligned with the terminal palette. |
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

Compiles to `fzt.wasm`. Sets `EnvTags: ["wasm", "browser"]` so browser-inappropriate commands (e.g. `update`) are filtered out of the palette. Exposes a global `fzt` object to JavaScript:

- `fzt.loadYAML(yaml)` -- parse YAML items
- `fzt.setFrontend({name, version})` -- register frontend identity
- `fzt.addCommands([{name, description, action}])` -- register frontend commands for `:` palette
- `fzt.setLabel(text)` -- set the top-left border label (e.g. user name after auth)
- `fzt.init(cols, rows)` -- create session, returns `{ansi, cursorX, cursorY}`
- `fzt.handleKey(key, ctrl, shift)` -- process keyboard event, returns frame + action
- `fzt.clickRow(row)` -- process mouse click on visual row
- `fzt.resize(cols, rows)` -- resize terminal
- `fzt.getVisibleRows()` / `fzt.getPromptState()` / `fzt.getUIState()` -- structured data API for DOM renderer

### `cmd/automate/` -- Shell automation binary

Standalone CLI tool: loads a YAML menu, presents an interactive tree picker via `tui.Run`, prints selected leaf to stdout. Shell wrappers execute the selection as a function. Sets `EnvTags: ["terminal"]` so terminal-only commands (e.g. `update`) appear in the palette.

Usage: `fzt-automate --yaml <path> [--title "..."] [--header "Name\tDescription"]`

### `build/` -- Build script

`go run ./build` builds the native automate binary with version injection via ldflags (`-X render.Version=<git describe>` and `-X terminal.EngineVersion=<fzt engine version from go.mod or git describe for local replace>`). `go run ./build wasm` builds the WASM binary.

## Style system

- **Palette**: Catppuccin Mocha. Terminal uses tcell color constants mapped to the 16-color palette. Browser uses the same hex values via `fzt-web.js` DEFAULT_PALETTE and CSS custom properties.
- **Font**: Perfect DOS VGA 437 (primary), Cascadia Code / Fira Code / JetBrains Mono (fallbacks). Nerd Font Mono for icons.
- **CRT effects** (browser only): scanline overlay, vignette, border-radius, font-smoothing disabled.

## Command palette

The `:` item is a hidden folder injected into every fzt app's item tree.

- **No frontend registered** (`FrontendName == ""`): `:` -> core commands (version toggle, validate, updatetimer, sync, update).
- **Frontend registered**: `:` -> frontend commands + `::` subfolder -> core commands. The frontend registers commands via `State.FrontendCommands` or WASM `fzt.addCommands()`.

The prompt shows scope breadcrumbs when inside command folders. `ScopeCtlTitle` returns "fzt ctl" at `::` depth or "<frontend> ctl" at `:` depth.

### Disambiguation of on/off leaves

The command tree may contain multiple items named "on" and "off" (e.g. under `whoami`). These are NOT ambiguous -- fzt's ancestor matching lets users type "whoami on" to reach the exact item. Note: `version` is now a single toggle leaf (not a folder with on/off children); selecting it flips TitleOverride and its description shows the version string. See `fzt/core/scorer.go` ScoreItem comment for the design rationale. Never rename on/off items to be unique.

### FrontendCommands nesting

`CommandItem.Children` enables one level of nesting in the `:` palette. Parent commands with children appear as folders; selecting a child returns its `Action` string. In `buildTwoLevelCommandTree`, index math reserves contiguous ranges: `idx++` for the parent, then `idx += len(cmd.Children)` for children. Items must be appended in the same order indices were reserved.

### Startup flow (terminal -- `tui.Run`)

1. Items loaded from `menu-cache.yaml` (synced from `/api/menu` endpoint) instead of static root.yaml
2. `tui.Run(items, cfg)` -- creates tcell screen, delegates to `runWithSession` if TreeMode
3. `core.NewState(items, cfg)` -- creates root context with AllItems
4. `applyFrontendConfig(s, cfg)` -- copies FrontendName/Version/Commands to State
5. `terminal.InjectCommandFolder(s, EngineVersion)` -- appends hidden `:` folder
6. `initSyncCheck` -- starts 1-second ticker goroutine that posts `tcell.EventInterrupt` for live redraws; checks bookmark staleness on a 20-minute interval
7. Render loop: `drawUnified` -> `PollEvent` -> `HandleUnifiedKey` -> `processAction` -> repeat. Title bar is a dynamic status area managed via `State.SetTitle`/`ClearTitle`.

### Startup flow (WASM -- `cmd/wasm/main.go`)

1. JS calls `fzt.loadYAML(yaml)` -> stores parsed items
2. JS calls `fzt.setFrontend({name, version})` -> buffers in `pendingFrontend`
3. JS calls `fzt.addCommands([...])` -> buffers in `pendingCommands`
4. JS calls `fzt.init(cols, rows)` -> creates session, applies pending config, injects commands
5. Subsequent: `fzt.handleKey(key, ctrl, shift)` -> returns `{ansi, action}`

The pending* variables buffer config because InjectCommandFolder runs during init and needs FrontendName/Commands already set on State.

### Action string format

All key/click handlers return an action string:

- `""` -- handled internally (version toggle, validate, updatetimer, sync, scope change)
- `"cancel"` -- user quit (Ctrl+C, Escape from root)
- `"select:<output>"` -- leaf selected. Output is the formatted item fields per AcceptNth.
- `"update"` -- user selected the fzt self-update command
- `"loaded"` -- load-* command completed (auto-syncs and exits)
- `"unloaded"` -- unload command completed (clears cache and exits)
- Any other string -- frontend command action (e.g., `"edit"`, `"copy-yaml"`)

### Cross-repo references

- Scoring engine (TieredScore, FuzzyMatch, FilterItems, ancestor matching): `fzt/core/scorer.go`, `fzt/core/tree.go`
- Ancestor matching design doc: `fzt/CLAUDE.md` "Ancestor matching eliminates name collisions"
- Homepage bookmark integration: `my-homepage/frontend/fzh-terminal.js`
- Config field semantics: `fzt/CLAUDE.md` "Config field relationships"

## Dependencies

- `github.com/nelsong6/fzt v0.1.30` -- engine (state, scoring, tree logic, YAML parsing, render abstractions)
- `github.com/gdamore/tcell/v2` -- terminal screen library
- `github.com/zalando/go-keyring` -- credential store (Windows Credential Manager / KWallet / macOS Keychain)
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

The build script injects `render.Version` from `git describe --tags --always --dirty` and `terminal.EngineVersion` from the fzt engine version in go.mod (or git describe for local replace) via ldflags.

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
- **Title bar as status area**: `State.SetTitle` / `ClearTitle` manage the title bar as a dynamic status area. TitleOverride always takes priority over ambient displays (timer, sync status). `drawBorderTopWithTitle` accepts titleStyleHint and syncIcon parameters.
- **Background sync**: Ticker goroutine in tui/sync.go posts `tcell.EventInterrupt` for live redraws without blocking the event loop.
- **Internalized shell commands**: load-*, unload, sync handled internally in HandleCommandAction -- no PowerShell function dependencies. load auto-syncs and exits; unload clears cache and exits.

## Change log

### 2026-04-15

1. **EnvTags / DisplayCondition documentation** -- Documented the environment-based command filtering system across three CLAUDE.md sections: root package (full mechanism), WASM bridge (tags set), automate (tags set). Previously undocumented despite being shipped in the prior session.
2. **setLabel WASM export documented** -- Added missing `fzt.setLabel(text)` to the WASM bridge API list. Was shipped in the 2026-04-05 session (border label + setLabel WASM export) but never added to the docs.

### 2026-04-09

1. **Header alignment fix** -- Headers now start at borderOffset+5 to match tree row layout (2 selection + 2 icon + 1 buffer). Removed Tiered-specific +2 compensation. Dynamic effectiveNameCol computed from visible items.
2. **Version rework** -- Version folder (with on/off children) replaced by single toggle leaf whose description shows the version string. Selecting toggles TitleOverride. EngineVersion now injected via ldflags in build script (no more "dev" fallback).
3. **Console output line** -- Title bar repurposed as dynamic status area. drawBorderTopWithTitle accepts titleStyleHint (0=default, 1=green, 2=red) and syncIcon parameters. TitleOverride always takes priority over ambient displays (timer etc).
4. **Background sync check** -- New tui/sync.go with initSyncCheck, checkBookmarkStaleness. 1-second ticker goroutine posts tcell.EventInterrupt for live redraws. 20-minute sync interval with .last-sync-check file.
5. **Credential store integration** -- New credential.go with HandleValidate and ReadJWTSecret using go-keyring (Windows Credential Manager / KWallet / macOS Keychain). Validate command in :: core palette.
6. **Internalized shell commands** -- load-*, unload, sync (formerly syncbookmarks) all handled internally in HandleCommandAction. No more PowerShell function dependencies. load auto-syncs and exits. unload clears cache and exits.
7. **API-backed menu** -- Menu tree now loaded from menu-cache.yaml (synced from /api/menu endpoint) instead of static root.yaml. New sync.go in root package with JWT minting, API fetching, YAML serialization. Menu API endpoints added to my-homepage routes.
8. **Build script EngineVersion injection** -- build/main.go now reads fzt engine version from go.mod (or git describe for local replace) and injects via ldflags alongside render.Version.
9. **updatetimer command** -- In :: core palette, toggles live countdown to next sync check in title bar.
10. **SyncIcon rendering** -- Yellow icon in top-right corner of border when sync is available.
11. **SetTitle/ClearTitle** -- All title bar writes now go through State.SetTitle/ClearTitle which evict ambient displays.

### 2026-04-10

1. **Shortcuts system** -- Shift as the single modifier namespace. Shift+HJKL (vim nav with arrow feedback), Shift+S (sync), Shift+W (save), Shift+A (add after), Shift+F (add folder), Shift+R/I (inspect/edit properties), Shift+D (delete). Shift+Enter confirms action modes. Shift+Backspace resets navigation to home. Unknown shortcuts show red `?` in title bar. Shortcuts folder in `:` palette for discoverability.
2. **Edit functionality** -- Add after, add folder, rename, delete, save via menu or shortcuts. Action mode pattern: enter mode, navigate, Shift+Enter confirms. Property inspection with gear icon items showing name/description/url/action. Inline text editing for property values. `●` dirty indicator in top-right when unsaved changes exist.
3. **Save to cloud** -- New SaveMenu function with PUT /api/menu. Conflict detection via baseVersion (409 on stale version). Menu version persisted to `.menu-version` sidecar file across restarts.
4. **Menu versioning** -- FetchMenu/SyncMenu now return version numbers. InitialMenuVersion plumbed from automate main.go through Config to State.MenuVersion.
5. **Data-driven command palette** -- InjectCommandFolder skips injection if `:` palette already exists in loaded cache. Action field on all command items for stable routing that survives renames. HandleCommandAction routes by Action with fallback to Fields[0].
6. **ItemAction refactor** -- All `Item.Action` string and `Item.URL` string references replaced with `*core.ItemAction{Type, Target}`. Command palette items use `cmdAction()` helper. Inspect properties decompose ItemAction into "url"/"action" display. Automate output collapses URL/Action check into single `item.Action.Target` print. Sync YAML marshaling preserves separate url/action keys for backwards compatibility.
