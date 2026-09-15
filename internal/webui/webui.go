// Package webui embeds the built Vue dashboard (web/, built via `npm run
// build` into this package's dist/ directory) so it can be served directly
// from the ChronosMonitor binary without any external static files.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Assets returns the embedded frontend build output, rooted at dist/.
func Assets() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
