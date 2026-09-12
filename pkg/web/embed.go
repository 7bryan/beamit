package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFiles embed.FS

// exposes the embededd assets rooted at "/static"
func StaticFS() (fs.FS, error) {
	return fs.Sub(staticFiles, "static")
}
