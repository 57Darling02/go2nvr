package ivideon

import (
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/ivideon"
)

func Init() {
	streams.HandleFunc("ivideon", ivideon.Dial)
}
