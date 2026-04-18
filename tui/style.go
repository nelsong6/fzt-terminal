package tui

import "github.com/gdamore/tcell/v2"

// Catppuccin Mocha palette — canonical RGB values shared by all renderers.
// Terminal uses tcell color constants below. Web uses hex via fzt-web.js.
// GDI renderers (picker) use ColorToRGB to get these values.
var PaletteRGB = map[tcell.Color][3]uint8{
	tcell.ColorBlack:    {0x45, 0x47, 0x5A}, // surface1
	tcell.ColorMaroon:   {0xEB, 0xA0, 0xAC},
	tcell.ColorGreen:    {0xA6, 0xE3, 0xA1},
	tcell.ColorOlive:    {0xF9, 0xE2, 0xAF}, // yellow
	tcell.ColorNavy:     {0x89, 0xB4, 0xFA}, // blue
	tcell.ColorPurple:   {0xF5, 0xC2, 0xE7}, // pink
	tcell.ColorTeal:     {0x94, 0xE2, 0xD5},
	tcell.ColorSilver:   {0xBA, 0xC2, 0xDE}, // subtext1
	tcell.ColorGray:     {0x58, 0x5B, 0x70}, // surface2
	tcell.ColorRed:      {0xF3, 0x8B, 0xA8},
	tcell.ColorLime:     {0xA6, 0xE3, 0xA1},
	tcell.ColorYellow:   {0xF9, 0xE2, 0xAF},
	tcell.ColorBlue:     {0x89, 0xB4, 0xFA},
	tcell.ColorFuchsia:  {0xCB, 0xA6, 0xF7}, // mauve
	tcell.ColorAqua:     {0x89, 0xDC, 0xEB}, // sky
	tcell.ColorWhite:    {0xCD, 0xD6, 0xF4}, // text
	tcell.ColorDarkBlue: {0x89, 0xB4, 0xFA}, // selection bg
	tcell.ColorDarkCyan: {0x94, 0xE2, 0xD5},
	tcell.ColorDarkGray: {0x58, 0x5B, 0x70},
}

// Default background and foreground RGB (Catppuccin Mocha base/text).
var (
	BaseBgRGB = [3]uint8{0x1E, 0x1E, 0x2E}
	TextFgRGB = [3]uint8{0xCD, 0xD6, 0xF4}
)

// ColorToRGB converts a tcell color to its Catppuccin Mocha RGB values.
// Handles named colors (via PaletteRGB), 256-color palette, and true color.
// Returns the default foreground for unknown colors.
func ColorToRGB(c tcell.Color) (r, g, b uint8) {
	if c == tcell.ColorDefault {
		return TextFgRGB[0], TextFgRGB[1], TextFgRGB[2]
	}
	if rgb, ok := PaletteRGB[c]; ok {
		return rgb[0], rgb[1], rgb[2]
	}
	// 256-color palette or true color — tcell.RGB() handles both
	cr, cg, cb := c.RGB()
	if cr < 0 {
		return TextFgRGB[0], TextFgRGB[1], TextFgRGB[2]
	}
	return uint8(cr), uint8(cg), uint8(cb)
}

// Font configuration — shared defaults for GDI and terminal renderers.
const (
	DefaultFontName = "FiraCode Nerd Font Mono"
	DefaultFontSize = 12 // points (matches Windows Terminal default)
)

// Semantic color constants for all fzt renderers.
// The terminal frontend uses these directly as tcell colors.
// The web frontend maps equivalent roles via CSS custom properties.
//
// CSS equivalents (Catppuccin Mocha palette):
//   SelectionBg    → palette[4] blue     → #89b4fa
//   HighlightFg    → palette[2] green    → #a6e3a1
//   FolderIconFg   → palette[3] yellow   → #f9e2af
//   FolderNameFg   → palette[6] cyan     → #94e2d5
//   FileIconFg     → palette[7] white    → #bac2de
//   BorderFg       → palette[8] gray     → #585b70
//   HintFg         → palette[8] gray     → #585b70
//   PromptActiveFg → palette[3] yellow   → #f9e2af
//   QueryFg        → palette[7] white    → #bac2de
//   GhostFg        → palette[8] gray     → #585b70
//   HeaderFg       → palette[6] cyan     → #94e2d5
//   NavModeFg      → palette[6] cyan     → #94e2d5
//   SearchModeFg   → palette[3] yellow   → #f9e2af
//   PromptSurfaceBg → 256-color #236     → #303030
//   TitleFg        → palette[6] cyan     → #94e2d5
var (
	SelectionBg    = tcell.ColorDarkBlue
	HighlightFg    = tcell.ColorGreen
	FolderIconFg   = tcell.ColorYellow
	FolderNameFg   = tcell.ColorDarkCyan
	FileIconFg     = tcell.ColorWhite
	BorderFg       = tcell.ColorDarkGray
	HintFg         = tcell.ColorDarkGray
	PromptActiveFg = tcell.ColorYellow
	QueryFg        = tcell.ColorWhite
	GhostFg        = tcell.ColorDarkGray
	HeaderFg       = tcell.ColorDarkCyan
	NavModeFg      = tcell.ColorDarkCyan
	SearchModeFg   = tcell.ColorYellow
	TitleFg        = tcell.ColorDarkCyan
	PromptSurfaceBg = tcell.ColorValid + 236 // 256-color: #303030
	SeparatorFg    = tcell.ColorDarkGray
	BreadcrumbFg   = tcell.ColorDarkCyan
	VersionFg      = tcell.ColorDarkGray
	LabelFg        = tcell.ColorDarkGray
	PathFg         = tcell.ColorDarkGray
	TitleSuccessFg = tcell.ColorGreen
	TitleErrorFg   = tcell.ColorRed
	TitleNeutralFg = tcell.ColorLightGray  // "registered but no action" — distinct from success
	SyncIconFg     = tcell.ColorYellow
)
