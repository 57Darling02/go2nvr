package gopro

import (
	"net/http"

	"github.com/57Darling02/go2nvr/internal/api"
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/core"
	"github.com/57Darling02/go2nvr/pkg/gopro"
)

func Init() {
	streams.HandleFunc("gopro", func(source string) (core.Producer, error) {
		return gopro.Dial(source)
	})

	api.HandleFunc("api/gopro", apiGoPro)
}

func apiGoPro(w http.ResponseWriter, r *http.Request) {
	var items []*api.Source

	for _, host := range gopro.Discovery() {
		items = append(items, &api.Source{Name: host, URL: "gopro://" + host})
	}

	api.ResponseSources(w, items)
}
