package debug

import (
	"github.com/57Darling02/go2nvr/internal/api"
)

func Init() {
	api.HandleFunc("api/stack", stackHandler)
}
