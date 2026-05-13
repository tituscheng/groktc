package webui

import (
	"embed"
	"io/fs"
	"log/slog"
)

//go:embed assets
var embeddedAssets embed.FS

// assetsFS exposes the assets/ directory at the FS root.
func assetsFS() fs.FS {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		// embed guarantees the directory exists at build time.
		slog.Error("assetsFS panic", "error", err)
		panic(err)
	}
	return sub
}
