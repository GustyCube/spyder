package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed static/index.html
var content embed.FS

func staticFS() fs.FS {
	sub, _ := fs.Sub(content, "static")
	return sub
}
