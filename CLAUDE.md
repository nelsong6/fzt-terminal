# fzt-terminal

Terminal renderer for fzt. Provides the tcell-based full-screen TUI that's consumed as a Go library by `fzt-automate`, `fzt-picker`, and `fzt-browser`'s headless session. Imports the fzt engine (`github.com/nelsong6/fzt`) for state and scoring, and `github.com/nelsong6/fzt-frontend` for the command palette and shared interaction behavior.

The other half of the old fzt-terminal — the browser renderer and the WASM bridge — now lives at `github.com/nelsong6/fzt-browser`. The shell-automation CLI lives at `github.com/nelsong6/fzt-automate`. The command palette + credential + API sync live at `github.com/nelsong6/fzt-frontend`. See the 2026-04-16 split history at the bottom.

## Architecture

```
fzt (engine)                    fzt-terminal (this repo)
  core.State, core.HandleKey      tui/                -- terminal renderer (tcell + raw TTY)
  core.TreeContext, scoring
  render.Canvas, render.Session

fzt-frontend (interaction layer)
  frontend.InjectCommandFolder    consumed by tui/ for palette injection
  frontend.HandleCommandAction    consumed by tui/ for action routing
  frontend.CheckBookmarkStaleness  consumed by tui/sync.go for background staleness check

Consumers of this repo's tui package:
  fzt-automate   (CLI)        -> tui.Run for the shell menu
  fzt-picker     (Windows)    -> tui.NewSession headless, rendered via CGo/GDI
  fzt-browser    (WASM)       -> tui.NewSession headless, rendered via ANSI in browser
```

## Package guide

### fzt-frontend (imported, not in this repo)

Shared frontend behavior lives in `github.com/nelsong6/fzt-frontend`:

Shared frontend behavior imported by every fzt app:

- **`InjectCommandFolder`** -- appends the hidden `:` command folder to the item tree. Single-level (`:` -> core commands) when no frontend is registered; two-level (`:` -> frontend commands + `::` -> core commands) when `FrontendName` is set. Skips injection if palette already exists in loaded cache (data-driven mode).
- **`HandleCommandAction`** -- routes leaf selections in the command tree by `Item.Action.Target` (stable command identifier) with fallback to `Fields[0]`. Handles version toggle, validate, updatetimer, sync, edit modes (add-after, add-folder, rename, delete, inspect, save), and internalized shell commands (load-*, unload). Frontend commands matched by name or action string.
- **`EngineVersion`** -- module-level var injected via ldflags from the build script (reads fzt engine version from go.mod or git describe for local replace).
- **`ApplyConfig`** -- sets frontend identity (name, version, commands) from Config onto State before command injection.
- **EnvTags / DisplayCondition** -- environment-based command filtering. `Config.EnvTags` declares the runtime capabilities (e.g. `["terminal"]` for automate, `["wasm", "browser"]` for the WASM bridge). During command tree construction (`buildCoreLevelCommandTree` / `buildTwoLevelCommandTree`), items with a non-empty `DisplayCondition` are skipped unless the condition string is present in `EnvTags`. Example: the `update` command has `DisplayCondition: "terminal"` so it only appears in the terminal palette, never in the browser. Tags are defined in `core.Config`, propagated to `core.State.EnvTags`, and checked via `hasEnvTag()`. The engine's `core.Item.DisplayCondition` field and `core.State.EnvTags` field are the underlying storage.
- **`HandleValidate` / `ReadJWTSecret`** (`credential.go`) -- credential store integration via go-keyring (Windows Credential Manager / KWallet / macOS Keychain). HandleValidate is invoked from the validate command in the `::` core palette.
- **`sync.go`** (root package) -- API-backed tree sync. Generic `FetchTree(token, treeID)` / `SaveTree(token, treeID, tree, baseVersion)` against `/fzt/tree/:id`. `SyncMenu` / `SaveMenu` are thin wrappers that call the generics with `MenuTreeID(claims.Sub)` (→ `"<sub>-menu"`). YAML serialization to `menu-cache.yaml`. Version number persisted to `.menu-version` for next launch.

### `tui/` -- Terminal renderer

Full-screen TUI rendering via tcell. Key files:

| File | Purpose |
|------|---------|
| `tui.go` | Entry points: `Run` (full-screen), `NewSession`/`NewTreeSession` (headless), `Simulate` (testing). Layout functions: `drawDefault`, `drawReverse`, `drawUnified` (tree mode). `drawBorderTopWithTitle` renders the title bar with titleStyleHint (0=default cyan, 1=green success, 2=red error, 3=neutral slate, 4=nav-mode NavModeFg, 5=search-mode SearchModeFg) and syncIcon parameters. |
| `tree.go` | Unified tree renderer: `drawUnified` draws prompt bar + tree as a single navigation surface. `drawTreeRow` renders individual rows with icons, indentation, match highlighting. Headers start at borderOffset+5 to match tree row layout (2 selection + 2 icon + 1 buffer). Dynamic effectiveNameCol computed from visible items. |
| `sync.go` | Background sync check: `initSyncCheck`, `checkBookmarkStaleness`. 1-second ticker goroutine posts `tcell.EventInterrupt` for live redraws. 20-minute sync interval with `.last-sync-check` file. SyncIcon rendering (yellow icon in top-right corner). |
| `style.go` | Semantic color constants (Catppuccin Mocha palette mapped to tcell colors). CSS equivalents documented in comments. Exports `PaletteRGB` (tcell color → RGB map), `ColorToRGB()` (any tcell color to RGB), `BaseBgRGB`/`TextFgRGB` (default bg/fg), `DefaultFontName`/`DefaultFontSize` (shared font config), `NavModeFg`/`SearchModeFg` (mode-indicator colors). Used by GDI renderers (picker) to stay aligned with the terminal palette. |
| `canvas.go` | `tcellCanvas` wraps `tcell.Screen` to satisfy `render.Canvas`. |
| `commands.go` | Currently empty (placeholder). |

### Browser rendering — moved

The `web/` folder (JS/CSS shim) and `cmd/wasm/` (WASM bridge) both moved to `github.com/nelsong6/fzt-browser` on 2026-04-16. That repo publishes `fzt.wasm` + the four JS/CSS files as release assets; `my-homepage` and `fzt-showcase` download from there. See the fzt-browser CLAUDE.md for the WASM API reference.

### Build script — removed

`build/` was deleted post-split since this repo produces no binaries. Build helpers for consumers live in their own repos (e.g. `fzt-automate`'s `.github/workflows/build.yml`).

## Style system

- **Palette**: Catppuccin Mocha. Terminal uses tcell color constants mapped to the 16-color palette. Browser uses the same hex values via `fzt-web.js` DEFAULT_PALETTE and CSS custom properties.
- **Font**: Perfect DOS VGA 437 (primary), Cascadia Code / Fira Code / JetBrains Mono (fallbacks). Nerd Font Mono for icons.
- **CRT effects** (browser only): scanline overlay, vignette, border-radius, font-smoothing disabled.

## Command palette

The `:` item is a hidden folder injected into every fzt app's item tree.

- **No frontend registered** (`FrontendName == ""`): `:` -> core commands (version toggle, validate, updatetimer, sync, update).
- **Frontend registered**: `:` -> frontend commands + `::` subfolder -> core commands. The frontend registers commands via `State.FrontendCommands` or WASM `fzt.addCommands()`.

The prompt shows scope breadcrumbs when inside command folders.

### Disambiguation of on/off leaves

The command tree may contain multiple items named "on" and "off" (e.g. under `whoami`). These are NOT ambiguous -- fzt's ancestor matching lets users type "whoami on" to reach the exact item. Note: `version` is now a single toggle leaf (not a folder with on/off children); selecting it flips TitleOverride and its description shows the version string. See `fzt/core/scorer.go` ScoreItem comment for the design rationale. Never rename on/off items to be unique.

### FrontendCommands nesting

`CommandItem.Children` enables one level of nesting in the `:` palette. Parent commands with children appear as folders; selecting a child returns its `Action` string. In `buildTwoLevelCommandTree`, index math reserves contiguous ranges: `idx++` for the parent, then `idx += len(cmd.Children)` for children. Items must be appended in the same order indices were reserved.

### Startup flow (terminal -- `tui.Run`)

1. Items loaded from `menu-cache.yaml` (synced from `/fzt/tree/<sub>-menu` endpoint) instead of static root.yaml
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
- `"cancel"` -- user quit (Escape from root)
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

## Keyboard model

Two explicit modes, search-as-default, Shift-is-Shift.

### Search mode (default)

Typing fills the query. Shift behaves as a normal keyboard modifier — capitals and shifted symbols (`!@#$%^&*(_)+`) are literal query characters. No Shift+letter shortcuts; no Ctrl/Alt/Meta shortcuts anywhere. Every printable key is input.

### Normal mode

Entered explicitly via `` ` `` (backtick — matches Quake/Source/VS Code "console" gesture) or implicitly by pressing an arrow key. Cursor is visible on a tree row; the prompt icon switches from magnifying glass (🔍) to arrow (→) to signal the mode change. Query is preserved — normal mode does not touch it.

Bound keys:

- `h` / `j` / `k` / `l` — vim nav (left / down / up / right). Lowercase only; capital HJKL is input, not nav.
- Arrow keys — same as hjkl.
- Every other letter — silent (future: dead-key hint, tracked by [fzt#11](https://github.com/nelsong6/fzt/issues/11)).

Exits to search:

- `/` — return to search, query preserved verbatim.
- `Backspace` — return to search, chop the last char of the query. Useful for "I want something like this but not exactly this."

### Why this layout

fzt's "fuzzy match is navigation" thesis means search is the primary interaction; typing fills the query and Enter commits. Shift-as-Shift keeps the keyboard's shifted characters available for search input (filenames with punctuation, mixed-case queries). Normal mode is a secondary state for when the user wants to nav a visible cursor — an explicit mode instead of a fuzzy auto-transition. One entry gesture (backtick, or any arrow), two exits (`/` preserve, Backspace chop).

Single-letter palette commands (sync, save, add, etc.) live in the `:` folder — fuzzy-matchable by name. The `:` folder is visible at root (not hidden) so it's discoverable by scanning, not just tribal knowledge. Shift+letter is NOT a shortcut namespace; that pattern was retired.

### Capital letter input

Currently unsupported in any fzt text-input context. Shift is Shift for symbols, but capitals for rename/property-edit mode don't have a clean mechanism yet (CapsLock works as an OS-level workaround). A future chord or sticky-caps mode is tracked by [my-homepage#23](https://github.com/nelsong6/my-homepage/issues/23).

### Replacements for retired Ctrl bindings

All Ctrl bindings have been removed from the engine and the picker (my-homepage#24). Replacements:

- Escape cancels (was Ctrl+C)
- Arrow keys navigate (was Ctrl+P/N)
- Home / End move cursor within the query (was Ctrl+A/E)
- Escape clears the query (was Ctrl+U)
- No direct replacement for "delete word" (was Ctrl+W) — backspace repeatedly, or Escape-to-clear + retype

New code MUST NOT introduce any Ctrl, Alt, or Meta binding.

## Dependencies

- `github.com/nelsong6/fzt` -- engine (state, scoring, tree logic, YAML parsing, render abstractions)
- `github.com/nelsong6/fzt-frontend` -- command palette, identity, action routing, credential store, API-backed menu sync
- `github.com/gdamore/tcell/v2` -- terminal screen library
- `golang.org/x/term` -- raw terminal mode
- `golang.org/x/sys` -- Windows console API

## Building

Nothing to build here — this repo is a Go library plus a Node.js route package. Consumers (`fzt-automate`, `fzt-picker`, `fzt-browser`) import `github.com/nelsong6/fzt-terminal/tui` and handle their own builds.

## CI

GitHub Actions workflow (`.github/workflows/build.yml`) on push to main is a tag-and-dispatch job:

1. **Tag**: auto-increment patch version and create a GitHub release (no asset uploads — this is just so Go consumers can `go get` at a specific version).
2. **Dispatch**: notify the three Go-module consumers (`fzt-picker`, `fzt-automate`, `fzt-browser`) via `repository_dispatch` so their own dispatch workflows can bump `fzt-terminal` in their go.mod. Uses GitHub App token from Azure Key Vault.

Menu CRUD used to live in `packages/routes/` here (`@nelsong6/fzt-terminal-routes`, mounted at `/at`). Retired 2026-04-18 — all tree CRUD now runs through `@nelsong6/fzt-frontend-routes` at `/fzt/tree/:id` (flat ids like `nelson-menu`). The package directory and its publish workflow were deleted.

## Key patterns

- **Headless sessions**: `tui.NewSession` / `tui.NewTreeSession` create sessions that render to `MemScreen` without a real terminal. Used by WASM bridge and `Simulate`.
- **Dual render paths**: ANSI-based (terminal + `fzt-terminal.js`) and structured data (DOM renderer via `getVisibleRows` etc.).
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
