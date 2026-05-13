package model

import (
	"github.com/fatih/color"
	"github.com/tituscheng/groktc/internal/style"
)

var (
	colorHeader  = style.Header
	colorCurrent = style.OK
	colorModel   = color.New(color.FgBlue, color.Bold).SprintFunc()
	colorWarn    = style.Skip
	colorMuted   = style.Muted
)
