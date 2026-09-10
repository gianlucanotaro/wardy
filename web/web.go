// Package web embeds the built frontend (web/dist, produced by `npm run
// build`) into the Go binary.
package web

import (
	"embed"
	"io/fs"
	"mime"
)

//go:embed all:dist
var distFS embed.FS

func init() {
	// Go's file server falls back to sniffing (text/plain for JSON-shaped
	// content) without this — .webmanifest isn't in its built-in MIME
	// table, and this app's runtime environment can't rely on an OS-level
	// mime.types entry for it either.
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		panic(err)
	}
}

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
