package tapo

import (
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/core"
	"github.com/57Darling02/go2nvr/pkg/kasa"
	"github.com/57Darling02/go2nvr/pkg/tapo"
)

func Init() {
	streams.HandleFunc("kasa", func(source string) (core.Producer, error) {
		return kasa.Dial(source)
	})

	streams.HandleFunc("tapo", func(source string) (core.Producer, error) {
		return tapo.Dial(source)
	})

	streams.HandleFunc("vigi", func(source string) (core.Producer, error) {
		return tapo.Dial(source)
	})
}
