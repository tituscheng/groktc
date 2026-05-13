package tui

import "github.com/fatih/color"

var (
	tuiHeader  = color.New(color.FgCyan, color.Bold).SprintFunc()
	tuiCursor  = color.New(color.FgHiCyan, color.Bold).SprintFunc()
	tuiCheck   = color.New(color.FgGreen, color.Bold).SprintFunc()
	tuiMuted   = color.New(color.FgHiBlack).SprintFunc()
	tuiPreview = color.New(color.FgHiBlack).SprintFunc()
)
