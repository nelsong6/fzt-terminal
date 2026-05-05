package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/nelsong6/fzt/core"
	"github.com/nelsong6/fzt/render"
)

// -- Unified renderer --

// drawUnified renders the prompt bar and tree. The tree is the single
// navigation surface -- no separate results section.
// Within this function, ctx.NavMode affects ONLY the prompt icon.
// All other rendering is mode-independent.
func drawUnified(c render.Canvas, s *core.State, cfg Config, w, startY, h int) {
	ctx := s.TopCtx()

	borderOffset := 0
	y := startY

	if cfg.Border {
		title := cfg.Title
		titleStyle := 0
		// StatesBannerOn overrides everything — the inspector is the user's
		// active tool, and a stale title would obscure the state readout.
		if s.StatesBannerOn {
			title = s.Describe()
			titleStyle = 0
		} else if s.TitleOverride != "" {
			// TitleOverride always takes priority — it's an explicit message
			title = s.TitleOverride
			titleStyle = s.TitleStyle
		} else if s.SyncTimerShown && s.SyncNextCheck > 0 {
			remaining := s.SyncNextCheck - time.Now().Unix()
			if remaining < 0 {
				remaining = 0
			}
			m := remaining / 60
			sec := remaining % 60
			title = fmt.Sprintf("next sync check: %dm %02ds", m, sec)
		}
		// Dirty indicator takes priority over sync icon
		icon := s.SyncIcon
		if s.Dirty {
			icon = "\u25cf" // ●
		}
		drawBorderTopWithTitle(c, w, y, title, cfg.TitlePos, titleStyle, s.IsPulsing(), icon, cfg.Label)
		y++
		borderOffset = 1
	}

	hasQuery := len(ctx.Query) > 0
	visible := core.TreeVisibleItems(s)

	// Compute effective name column width from visible items.
	// Each row needs: indent + name + gap. We find the max absolute width
	// (indent + max(nameLen+1, NameColWidth+ColGap-indent)) so all descriptions align.
	effectiveNameCol := ctx.NameColWidth + ctx.ColGap
	if effectiveNameCol < 12 {
		effectiveNameCol = 12 // minimum so headers don't collapse
	}
	for _, row := range visible {
		indent := row.Item.Depth * 2
		nameW := ctx.NameColWidth + ctx.ColGap - indent
		if len(row.Item.Fields) > 0 {
			nameLen := len([]rune(row.Item.Fields[0])) + 1
			if nameLen > nameW {
				nameW = nameLen
			}
		}
		if nameW+indent > effectiveNameCol {
			effectiveNameCol = nameW + indent
		}
	}

	// Prompt bar -- bordered input field, the primary UI element
	promptBg := PromptSurfaceBg
	borderStyle := tcell.StyleDefault.Foreground(BorderFg)

	// Mode indicator: search (magnifying glass) vs nav (arrow) vs inspect (gear)
	var promptIcon rune
	var promptIconStyle tcell.Style
	if s.PromptMode != "" {
		promptIcon = s.PromptIcon
		if promptIcon == 0 {
			promptIcon = '\uF002'
		}
		promptIconStyle = tcell.StyleDefault.Foreground(SearchModeFg).Bold(true).Background(promptBg)
	} else if s.InspectTargetIdx >= 0 {
		promptIcon = '\u2699' // ⚙ gear for inspect mode
		promptIconStyle = tcell.StyleDefault.Foreground(HintFg).Background(promptBg)
	} else if ctx.NavMode {
		promptIcon = '\uF0A9' //
		promptIconStyle = tcell.StyleDefault.Foreground(NavModeFg).Background(promptBg)
	} else {
		promptIcon = '\uF002' //
		promptIconStyle = tcell.StyleDefault.Foreground(SearchModeFg).Bold(true).Background(promptBg)
	}
	promptLen := 2 // icon + space

	// Top border of prompt bar
	c.SetContent(borderOffset, y, '\u250c', nil, borderStyle) // +
	for x := borderOffset + 1; x < w-borderOffset-1; x++ {
		c.SetContent(x, y, '\u2500', nil, borderStyle) // -
	}
	c.SetContent(w-borderOffset-1, y, '\u2510', nil, borderStyle) // +
	y++

	// Prompt content line with background
	c.SetContent(borderOffset, y, '\u2502', nil, borderStyle) // |
	for x := borderOffset + 1; x < w-borderOffset-1; x++ {
		c.SetContent(x, y, ' ', nil, tcell.StyleDefault.Background(promptBg))
	}
	c.SetContent(w-borderOffset-1, y, '\u2502', nil, borderStyle) // |

	px := borderOffset + 1       // content starts inside the border
	pw := w - borderOffset*2 - 2 // content width inside borders
	// Prompt: [icon] [locked breadcrumb >] [query or nav preview]
	c.SetContent(px, y, promptIcon, nil, promptIconStyle)
	c.SetContent(px+1, y, ' ', nil, tcell.StyleDefault.Background(promptBg))
	tx := px + promptLen // text position after icon + space

	// Context breadcrumb -- ':' when in a pushed context (command mode)
	scopeLen := 0
	if len(s.Contexts) > 1 && ctx.PromptIcon != 0 {
		lockedStyle := tcell.StyleDefault.Foreground(PathFg).Background(promptBg)
		c.SetContent(tx+scopeLen, y, ctx.PromptIcon, nil, lockedStyle)
		scopeLen++
		c.SetContent(tx+scopeLen, y, ' ', nil, tcell.StyleDefault.Background(promptBg))
		scopeLen++
	}

	// Scope breadcrumb -- just the word greyed out with a space after it.
	if len(ctx.Scope) > 1 {
		lockedStyle := tcell.StyleDefault.Foreground(PathFg).Background(promptBg)
		for si := 1; si < len(ctx.Scope); si++ {
			level := ctx.Scope[si]
			if level.ParentIdx >= 0 && level.ParentIdx < len(ctx.AllItems) {
				name := ctx.AllItems[level.ParentIdx].Fields[0]
				drawText(c, tx+scopeLen, y, name, lockedStyle, pw-promptLen-scopeLen)
				scopeLen += len([]rune(name))
				c.SetContent(tx+scopeLen, y, ' ', nil, lockedStyle)
				scopeLen++
			}
		}
	}

	qx := tx + scopeLen // where editable query starts

	contentX := qx // where query or nav preview starts
	contentW := pw - promptLen - scopeLen

	if s.PromptMode != "" {
		queryStyle := tcell.StyleDefault.Foreground(QueryFg).Background(promptBg)
		if len(s.PromptQuery) > 0 {
			drawText(c, contentX, y, string(s.PromptQuery), queryStyle, contentW)
		} else {
			hint := s.PromptPlaceholder
			if hint == "" {
				hint = "search\u2026"
			}
			hintStyle := tcell.StyleDefault.Foreground(HintFg).Italic(true).Background(promptBg)
			drawText(c, contentX, y, hint, hintStyle, contentW)
		}
		c.ShowCursor(contentX+s.PromptCursor, y)
	} else if s.EditMode == "rename" {
		// Rename mode — show edit buffer as editable text
		renameStyle := tcell.StyleDefault.Foreground(QueryFg).Background(promptBg)
		drawText(c, contentX, y, string(s.EditBuffer), renameStyle, contentW)
		c.ShowCursor(contentX+len(s.EditBuffer), y)
	} else if hasQuery {
		queryStyle := tcell.StyleDefault.Foreground(QueryFg).Background(promptBg)
		drawText(c, contentX, y, string(ctx.Query), queryStyle, contentW)
		c.ShowCursor(contentX+ctx.Cursor, y)

		// Ghost autocomplete text: show remaining chars of top match if query is a prefix
		if ctx.Cursor == len(ctx.Query) && len(ctx.Filtered) > 0 && len(ctx.Filtered[0].Fields) > 0 {
			name := ctx.Filtered[0].Fields[0]
			nameRunes := []rune(name)
			if len(nameRunes) > len(ctx.Query) && strings.EqualFold(string(nameRunes[:len(ctx.Query)]), string(ctx.Query)) {
				ghost := string(nameRunes[len(ctx.Query):])
				ghostStyle := tcell.StyleDefault.Foreground(GhostFg).Background(promptBg)
				drawText(c, contentX+len(ctx.Query), y, ghost, ghostStyle, contentW-len(ctx.Query))
			}
		}
	} else if ctx.SearchActive || len(ctx.Scope) > 1 {
		hintStyle := tcell.StyleDefault.Foreground(HintFg).Italic(true).Background(promptBg)
		drawText(c, qx, y, "search\u2026", hintStyle, pw-promptLen-scopeLen)
		c.ShowCursor(qx, y)
	} else {
		hintStyle := tcell.StyleDefault.Foreground(HintFg).Italic(true).Background(promptBg)
		drawText(c, qx, y, "type to search\u2026", hintStyle, pw-promptLen-scopeLen)
		c.ShowCursor(qx, y)
	}
	y++

	// Bottom border of prompt bar
	c.SetContent(borderOffset, y, '\u2514', nil, borderStyle) // +
	for x := borderOffset + 1; x < w-borderOffset-1; x++ {
		c.SetContent(x, y, '\u2500', nil, borderStyle) // -
	}
	c.SetContent(w-borderOffset-1, y, '\u2518', nil, borderStyle) // +
	y++

	// Headers
	if len(ctx.Headers) > 0 {
		hdrStyle := tcell.StyleDefault.Foreground(HeaderFg).Bold(true)
		x := borderOffset + 5 // match tree row layout: 2 selection + 2 icon + 1 buffer
		for fi, hdr := range ctx.Headers[0].Fields {
			colW := effectiveNameCol
			if fi > 0 {
				colW = 0
			}
			drawText(c, x, y, hdr, hdrStyle, w-x-borderOffset)
			x += colW
		}
		y++

		divStyle := tcell.StyleDefault.Foreground(SeparatorFg)
		for x := borderOffset + 1; x < w-borderOffset; x++ {
			c.SetContent(x, y, '\u2500', nil, divStyle)
		}
		y++
	}

	// Tree section -- the single navigation surface
	totalSpace := h - (y - startY) - borderOffset
	treeSpace := totalSpace

	// When query active, find top match in tree for highlighting
	topMatchIdx := -1
	if hasQuery && len(ctx.Filtered) > 0 {
		topMatchIdx = core.FindInAll(ctx.AllItems, ctx.Filtered[0])
	}

	// Scroll tree to keep cursor visible
	if ctx.TreeCursor >= 0 {
		if ctx.TreeCursor < ctx.TreeOffset {
			ctx.TreeOffset = ctx.TreeCursor
		}
		if ctx.TreeCursor >= ctx.TreeOffset+treeSpace {
			ctx.TreeOffset = ctx.TreeCursor - treeSpace + 1
		}
	}
	if ctx.TreeOffset < 0 {
		ctx.TreeOffset = 0
	}

	for i := 0; i < treeSpace; i++ {
		vi := ctx.TreeOffset + i
		if vi >= len(visible) {
			break
		}
		row := visible[vi]
		isSelected := vi == ctx.TreeCursor
		isTopMatch := hasQuery && row.ItemIdx == topMatchIdx && !isSelected
		drawTreeRow(c, row, isSelected, isTopMatch, ctx, cfg, borderOffset, y+i, w, effectiveNameCol)
	}

	if cfg.Border {
		drawBorderBottom(c, w, startY+h-1)
		drawBorderSides(c, w, startY, startY+h-1)
	}
}

// drawTreeRow renders a single tree item row.
func drawTreeRow(c render.Canvas, row core.TreeRow, isSelected, isTopMatch bool, ctx *core.TreeContext, cfg Config, borderOffset, y, w, effectiveNameCol int) {
	// Fill background
	if isSelected || isTopMatch {
		bg := tcell.StyleDefault.Background(SelectionBg)
		for x := borderOffset; x < w-borderOffset; x++ {
			c.SetContent(x, y, ' ', nil, bg)
		}
	}

	x := borderOffset
	hasBg := isSelected || isTopMatch

	// Selection indicator
	if isSelected {
		indStyle := tcell.StyleDefault.Foreground(FolderIconFg).Bold(true).Background(SelectionBg)
		drawText(c, x, y, "\uf054 ", indStyle, 2)
	} else {
		style := tcell.StyleDefault
		if hasBg {
			style = style.Background(SelectionBg)
		}
		drawText(c, x, y, "  ", style, 2)
	}
	x += 2

	// Indentation
	indent := row.Item.Depth * 2
	for i := 0; i < indent; i++ {
		style := tcell.StyleDefault
		if hasBg {
			style = style.Background(SelectionBg)
		}
		c.SetContent(x+i, y, ' ', nil, style)
	}
	x += indent

	// Icon
	var iconRune rune
	var iconStyle tcell.Style
	if row.Item.IsProperty {
		iconRune = '\u2699' // ⚙ gear icon for property items
		iconStyle = tcell.StyleDefault.Foreground(HintFg)
	} else if row.Item.HasChildren {
		iconRune = '\U000F024B'
		iconStyle = tcell.StyleDefault.Foreground(FolderIconFg).Bold(true)
	} else {
		iconRune = '\uF15B'
		iconStyle = tcell.StyleDefault.Foreground(FileIconFg)
	}
	if hasBg {
		iconStyle = iconStyle.Background(SelectionBg)
	}
	c.SetContent(x, y, iconRune, nil, iconStyle)
	x += 2 // wide icon occupies 2 cells
	bufStyle := tcell.StyleDefault
	if hasBg {
		bufStyle = bufStyle.Background(SelectionBg)
	}
	c.SetContent(x, y, ' ', nil, bufStyle)
	x++

	// Name
	name := ""
	if len(row.Item.Fields) > 0 {
		name = row.Item.Fields[0]
	}
	var nameStyle tcell.Style
	if row.Item.HasChildren {
		nameStyle = tcell.StyleDefault.Foreground(FolderNameFg).Bold(true)
	} else if isSelected {
		nameStyle = tcell.StyleDefault.Foreground(QueryFg)
	} else {
		nameStyle = tcell.StyleDefault
	}
	if hasBg {
		nameStyle = nameStyle.Background(SelectionBg)
	}

	nameWidth := effectiveNameCol - indent
	nameRunes := []rune(name)
	if nameWidth < len(nameRunes)+1 {
		nameWidth = len(nameRunes) + 1
	}

	// Highlight matched characters for top match
	if isTopMatch && len(row.Item.MatchIndices) > 0 && len(row.Item.MatchIndices[0]) > 0 {
		drawHighlightedText(c, x, y, name, nameStyle, nameWidth, row.Item.MatchIndices[0], hasBg)
	} else {
		drawText(c, x, y, name, nameStyle, nameWidth)
	}
	x += nameWidth

	// Description
	if len(row.Item.Fields) > 1 {
		desc := row.Item.Fields[1]
		descStyle := tcell.StyleDefault
		if hasBg {
			descStyle = descStyle.Background(SelectionBg)
		}
		remaining := w - x - borderOffset
		if remaining > 0 {
			drawText(c, x, y, desc, descStyle, remaining)
		}
	}
}

// drawHighlightedText draws text with certain character indices highlighted in green.
func drawHighlightedText(c render.Canvas, x, y int, text string, baseStyle tcell.Style, maxW int, matchIndices []int, hasBg bool) {
	runes := []rune(text)
	matchSet := make(map[int]bool, len(matchIndices))
	for _, idx := range matchIndices {
		matchSet[idx] = true
	}

	for i, r := range runes {
		if i >= maxW {
			break
		}
		style := baseStyle
		if matchSet[i] {
			style = tcell.StyleDefault.Foreground(HighlightFg).Bold(true)
			if hasBg {
				style = style.Background(SelectionBg)
			}
		}
		c.SetContent(x+i, y, r, nil, style)
	}
}
