package api

import (
	"io/fs"
	"net/http"

	"github.com/AlexxIT/go2rtc/www"
)

func initStatic(staticDir string, embedded fs.FS) {
	root := staticRoot(staticDir, embedded)

	base := len(basePath)
	fileServer := http.FileServer(root)

	HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
		if base > 0 {
			r.URL.Path = r.URL.Path[base:]
		}
		fileServer.ServeHTTP(w, r)
	})
}

func staticRoot(staticDir string, embedded fs.FS) http.FileSystem {
	if staticDir != "" {
		log.Info().Str("dir", staticDir).Msg("[api] serve static")
		return http.Dir(staticDir)
	}
	if embedded != nil {
		return http.FS(embedded)
	}
	return http.FS(www.Static)
}
