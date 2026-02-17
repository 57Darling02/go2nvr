package eseecloud

import (
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/eseecloud"
)

func Init() {
	streams.HandleFunc("eseecloud", eseecloud.Dial)
}
