package flussonic

import (
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/flussonic"
)

func Init() {
	streams.HandleFunc("flussonic", flussonic.Dial)
}
