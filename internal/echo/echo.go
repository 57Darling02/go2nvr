package echo

import (
	"bytes"
	"errors"
	"os/exec"
	"slices"

	"github.com/57Darling02/go2nvr/internal/app"
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/shell"
)

func Init() {
	var cfg struct {
		Mod struct {
			AllowPaths []string `yaml:"allow_paths"`
		} `yaml:"echo"`
	}

	app.LoadConfig(&cfg)

	allowPaths := cfg.Mod.AllowPaths

	log := app.GetLogger("echo")

	streams.RedirectFunc("echo", func(url string) (string, error) {
		args := shell.QuoteSplit(url[5:])

		if allowPaths != nil && !slices.Contains(allowPaths, args[0]) {
			return "", errors.New("echo: bin not in allow_paths: " + args[0])
		}

		b, err := exec.Command(args[0], args[1:]...).Output()
		if err != nil {
			return "", err
		}

		b = bytes.TrimSpace(b)

		log.Debug().Str("url", url).Msgf("[echo] %s", b)

		return string(b), nil
	})
	streams.MarkInsecure("echo")
}
