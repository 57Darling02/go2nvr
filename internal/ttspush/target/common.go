package target

import (
	"context"
	"io"
)

type Generator func(context.Context) (io.ReadCloser, error)

type IPWebcamOptions struct {
	URL         string
	SampleRate  int
	ChunkMS     int
	Realtime    bool
	InsecureTLS bool
}

type BackchannelOptions struct {
	Dst string
}
