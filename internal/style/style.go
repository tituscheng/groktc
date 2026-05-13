// Package style provides terminal color styling helpers used across commands.
package style

import "github.com/fatih/color"

var (
	Header  = color.New(color.FgCyan, color.Bold).SprintFunc()
	Label   = color.New(color.FgHiWhite, color.Bold).SprintFunc()
	OK      = color.New(color.FgGreen, color.Bold).SprintFunc()
	Skip    = color.New(color.FgYellow, color.Bold).SprintFunc()
	Fail    = color.New(color.FgRed, color.Bold).SprintFunc()
	Muted   = color.New(color.FgHiBlack).SprintFunc()
	Value   = color.New(color.FgGreen).SprintFunc()
	Total   = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	Warning = color.New(color.FgYellow).SprintFunc()
)
