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

- **`InjectCommandFolder`** -- appends the `:` command folder to the item tree. Single-level (`:` -> core commands) when no frontend is registered; two-level (`:` -> frontend commands + `::` -> core commands) when `FrontendName` is set. The `:` root is visible at root since 2026-04-19 (discoverability over stealth; was previously `Hidden: true`). Skips injection entirely if `Config.HidePalette` is set (e.g. unauthenticated homepage visitors) or if a `:` palette already exists in the loaded cache (data-driven mode).
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
4. `applyFrontendConfig(s, cfg)` -- copies FrontendName/Version/Commands + HidePalette to State
5. `terminal.InjectCommandFolder(s, EngineVersion)` -- appends the `:` palette folder (visible at root; skipped entirely when `HidePalette` is set)
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

### States

fzt has two primary modes and several edit modes. The primary modes are mutually exclusive. Edit modes layer on top and disable most of the primary mode's keys until the edit is confirmed or canceled.

**Search mode** (default). Typing fills the query; Enter commits the top match or scopes into the top folder; arrows or backtick enter normal mode. Prompt icon: magnifying glass (`\uF002`, yellow).

**Normal mode** (cursor on tree). Typing is no longer input; lowercase `hjkl` navigates; `/` or Backspace return to search. Prompt icon: arrow (`\uF0A9`, teal).

**Edit modes** (`State.EditMode` non-empty). Entered from palette commands (`rename`, `add-after`, `add-folder`, `delete`, `inspect`). Under rename or property-edit, every printable key goes to `EditBuffer` via `handleRenameKey`; Enter confirms, Escape cancels. Under the other edit modes, the user navigates to a target and confirms with `Shift+Enter`.

Additional ambient flags: `StatesBannerOn` (action-preview inspector; suppresses leaf execution), `InspectTargetIdx >= 0` (inspection property rows visible under the target). Neither changes key meaning significantly but both affect rendering.

### Key table, by mode

Legend: ✓ = handled, ▸ = has side effect, — = unbound / silent, ↩ = transitions to another state.

| key | search | normal | rename / property-edit |
|---|---|---|---|
| printable rune (incl. capital, symbol) | ✓ append to query | — silent (except `h`/`j`/`k`/`l` / `/`) | ✓ append to `EditBuffer` |
| `h`/`j`/`k`/`l` (lowercase) | ✓ append to query | ✓ nav left/down/up/right | ✓ append to buffer |
| `/` | ✓ activate search from root (no-op if already active) | ↩ return to search, query preserved | ✓ append `/` to buffer |
| `` ` `` (backtick) | ↩ enter normal mode | — | ✓ append backtick to buffer |
| `:` | ✓ append `:` to query (matches `:` folder at root via fuzzy) | — | ✓ append to buffer |
| Space | ▸ on folder at cursor: push scope; else append to query | ▸ push scope on cursor's folder | ✓ append space to buffer |
| Arrow keys | ↩ enter normal mode + move cursor | ✓ move cursor | — |
| Tab | ✓ autocomplete top match (folder: also push scope) | — | — |
| Enter | ✓ select top match or scoped leaf; push scope on folder | ✓ same | ✓ confirm edit |
| Shift+Enter | ✓ universal confirm-select (commits cursor's item, no scope push on folder) | ✓ same | ✓ confirm edit mode action (add / rename / delete / inspect) |
| Backspace | ✓ delete last query char; on empty query, pop scope | ↩ chop last query char + return to search | ✓ delete last char of buffer |
| Shift+Backspace | ▸ reset navigation to home (pop all scope) | ▸ same | ▸ same; preserves edit mode |
| Home / End | ✓ move query cursor to start / end | — | — |
| Escape | ▸ unwind cascade (see below) | ▸ unwind cascade | ↩ cancel edit, return to prior mode |

### Back-out / unwind cascade

Escape performs progressive unwind. Each press steps back one layer; reaching root exits the picker.

1. **Edit / inspect mode active** → cancel the edit, clear `EditMode`, clean up any property rows, return to the prior mode (search or normal).
2. **Non-empty query** → clear the query; stay in search mode.
3. **Scope depth > 1** → pop one scope level.
4. **Context stack > 1** (e.g. scoped into `:` palette as a pushed context) → pop one context.
5. **At root with empty query, nothing to unwind** → `s.Cancelled = true`, picker returns "cancel".

Shift+Backspace collapses steps 2–4 into a single gesture — resets to home (pop all scope, clear query, clear filtered). Preserves the active edit mode when one is set, so the user doesn't lose an in-progress add/rename while re-orienting.

Backspace on an empty query at scope > 1 does step 3 (pop scope) without needing Escape. Backspace on an empty query at root with context stack > 1 pops a context.

### Modifier policy

- **Shift**: keyboard modifier for capitals and shifted symbols. Passes through as the literal character. Not a shortcut namespace.
- **Ctrl**: no bindings anywhere. Removed in `my-homepage#24`; see "Retired Ctrl" below.
- **Alt / Meta**: never used.

Why: search scoring is case-insensitive so capitals and lowercase produce identical matches, but users still need symbols and — when editing — actual capitals. Reserving Shift globally for a shortcut namespace cost more than it saved. `:` palette commands are fuzzy-matchable by name, so "save" + Enter substitutes for what used to be Shift+W.

### Capital letter input

Currently typeable in any edit buffer (rename / property-edit / add-*) because `handleRenameKey` accepts every rune. In search mode they also go into the query; case-insensitive scoring makes them behave identically to lowercase. CapsLock works as an OS-level workaround when the keyboard layout can't produce the character otherwise. A future chord or sticky-caps mode is tracked by [my-homepage#23](https://github.com/nelsong6/my-homepage/issues/23).

### Retired Ctrl bindings

All Ctrl bindings have been removed from the engine and the picker. Replacements:

| was | now |
|---|---|
| Ctrl+C (cancel) | Escape from root |
| Ctrl+P / Ctrl+N (nav) | Arrow keys, or lowercase `k` / `j` in normal mode |
| Ctrl+A / Ctrl+E (line nav) | Home / End |
| Ctrl+U (clear query) | Escape on non-empty query |
| Ctrl+W (delete word) | Backspace repeatedly, or Escape to clear and retype |

### Shortcut manifest

The canonical list of key → action mappings lives in `fzt-frontend/command.go`'s `helpEntries()` function, rendered into the `:help/` palette subfolder at runtime. Every fzt consumer gets the same list; users can scope into `:help/` (or fuzzy-match `help` from root) to browse every binding with its long-form description. Each entry carries `Action = "help-entry"`, which `HandleCommandAction` maps to a title-bar pulse so pressing Enter on a help row echoes the description.

This doc is the narrative reference. The palette is the runtime reference. A visual state-transition diagram is tracked by [fzt#10](https://github.com/nelsong6/fzt/issues/10) and will live under `diagrams.romaine.life/fzt/keyboard` when built.

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

Two manually-triggered workflows (Claude-driven):

- **`.github/workflows/dispatch.yml`** (`workflow_dispatch`) — dependency bump path. Calls `go-dependency-update-template` to bump `github.com/nelsong6/fzt` + `github.com/nelsong6/fzt-frontend`, commit, push, and cut a release at the new sha.
- **`.github/workflows/release.yml`** (`workflow_dispatch`) — direct-push release path for code changes. Calls `release-and-dispatch-template` to tag the current main. Shared `release-tag` concurrency group with `dispatch.yml`.

Both just cut a Go-module release (no asset uploads — consumers `go get` at the tag). Downstream Go-module consumers (`fzt-picker`, `fzt-automate`, `fzt-browser`) are updated manually per repo — no `repository_dispatch` fan-out.

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
