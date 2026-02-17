package multitrans

import (
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/multitrans"
)

func Init() {
	streams.HandleFunc("multitrans", multitrans.Dial)
}
