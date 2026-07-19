package ffmpeg

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseQuery(t *testing.T) {
	args := parseQuery(nil)
	require.Equal(t, `ffmpeg -hide_banner -i - -c:v mjpeg -f mjpeg -`, args.String())

	query, err := url.ParseQuery("h=480")
	require.Nil(t, err)
	args = parseQuery(query)
	require.Equal(t, `ffmpeg -hide_banner -i - -c:v mjpeg -vf "scale=-1:480" -f mjpeg -`, args.String())

	query, err = url.ParseQuery("hw=vaapi")
	require.Nil(t, err)
	args = parseQuery(query)
	require.Equal(t, `ffmpeg -hide_banner -hwaccel vaapi -hwaccel_output_format vaapi -hwaccel_flags allow_profile_mismatch -i - -c:v mjpeg_vaapi -vf "format=vaapi|nv12,hwupload" -f mjpeg -`, args.String())
}

func TestJPEGWithScaleContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := JPEGWithScaleContext(ctx, nil, 640, -1)
	require.ErrorIs(t, err, context.Canceled)
}
