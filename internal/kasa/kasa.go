package kasa

import (
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/core"
	"github.com/57Darling02/go2nvr/pkg/kasa"
)

func Init() {
	streams.HandleFunc("kasa", func(source string) (core.Producer, error) {
		return kasa.Dial(source)
	})
}
