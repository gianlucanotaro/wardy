// Package web embeds the built frontend (web/dist, produced by `npm run
// build`) into the Go binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded frontend build rooted at dist/, so index.html is
// at the filesystem root rather than under a "dist" prefix.
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// dist is embedded at compile time from a directory that must exist
		// for the build to succeed at all — this cannot fail at runtime.
		panic(err)
	}
	return sub
}
