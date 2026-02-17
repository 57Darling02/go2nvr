package xiaomi

import (
	"strings"

	"github.com/57Darling02/go2nvr/pkg/core"
	"github.com/57Darling02/go2nvr/pkg/xiaomi/legacy"
	"github.com/57Darling02/go2nvr/pkg/xiaomi/miss"
)

func Dial(rawURL string) (core.Producer, error) {
	// Format: xiaomi/miss
	if strings.Contains(rawURL, "vendor") {
		return miss.Dial(rawURL)
	}

	// Format: xiaomi/legacy
	return legacy.Dial(rawURL)
}

func IsLegacy(model string) bool {
	return legacy.Supported(model)
}
