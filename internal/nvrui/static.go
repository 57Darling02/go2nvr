// Package nvrui embeds the go2nvr dashboard built from webui/.
package nvrui

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

var staticFS = func() fs.FS {
	root, err := fs.Sub(files, "dist")
	if err != nil {
		panic(fmt.Sprintf("nvrui: embedded static directory: %v", err))
	}
	return root
}()

// StaticFS returns the dashboard filesystem without its dist/ prefix.
func StaticFS() fs.FS {
	return staticFS
}
