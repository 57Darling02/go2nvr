package main

import (
	"slices"

	"github.com/57Darling02/go2nvr/internal/alsa"
	"github.com/57Darling02/go2nvr/internal/api"
	"github.com/57Darling02/go2nvr/internal/api/ws"
	"github.com/57Darling02/go2nvr/internal/app"
	"github.com/57Darling02/go2nvr/internal/bubble"
	"github.com/57Darling02/go2nvr/internal/debug"
	"github.com/57Darling02/go2nvr/internal/doorbird"
	"github.com/57Darling02/go2nvr/internal/dvrip"
	"github.com/57Darling02/go2nvr/internal/echo"
	"github.com/57Darling02/go2nvr/internal/eseecloud"
	"github.com/57Darling02/go2nvr/internal/exec"
	"github.com/57Darling02/go2nvr/internal/expr"
	"github.com/57Darling02/go2nvr/internal/ffmpeg"
	"github.com/57Darling02/go2nvr/internal/flussonic"
	"github.com/57Darling02/go2nvr/internal/gopro"
	"github.com/57Darling02/go2nvr/internal/hass"
	"github.com/57Darling02/go2nvr/internal/hls"
	"github.com/57Darling02/go2nvr/internal/homekit"
	"github.com/57Darling02/go2nvr/internal/http"
	"github.com/57Darling02/go2nvr/internal/isapi"
	"github.com/57Darling02/go2nvr/internal/ivideon"
	"github.com/57Darling02/go2nvr/internal/kasa"
	"github.com/57Darling02/go2nvr/internal/mjpeg"
	"github.com/57Darling02/go2nvr/internal/mp4"
	"github.com/57Darling02/go2nvr/internal/mpeg"
	"github.com/57Darling02/go2nvr/internal/multitrans"
	"github.com/57Darling02/go2nvr/internal/nest"
	"github.com/57Darling02/go2nvr/internal/ngrok"
	"github.com/57Darling02/go2nvr/internal/onvif"
	"github.com/57Darling02/go2nvr/internal/pinggy"
	"github.com/57Darling02/go2nvr/internal/record"
	"github.com/57Darling02/go2nvr/internal/ring"
	"github.com/57Darling02/go2nvr/internal/roborock"
	"github.com/57Darling02/go2nvr/internal/rtmp"
	"github.com/57Darling02/go2nvr/internal/rtsp"
	"github.com/57Darling02/go2nvr/internal/srtp"
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/internal/tapo"
	"github.com/57Darling02/go2nvr/internal/tuya"
	"github.com/57Darling02/go2nvr/internal/v4l2"
	"github.com/57Darling02/go2nvr/internal/webrtc"
	"github.com/57Darling02/go2nvr/internal/webtorrent"
	"github.com/57Darling02/go2nvr/internal/wyoming"
	"github.com/57Darling02/go2nvr/internal/wyze"
	"github.com/57Darling02/go2nvr/internal/xiaomi"
	"github.com/57Darling02/go2nvr/internal/yandex"
	"github.com/57Darling02/go2nvr/pkg/shell"
)

func main() {
	// version will be set later from -buildvcs info, this used only as fallback
	app.Version = "0.1.0"

	type module struct {
		name string
		init func()
	}

	modules := []module{
		{"", app.Init},    // init config and logs
		{"api", api.Init}, // init API before all others
		{"ws", ws.Init},   // init WS API endpoint
		{"", streams.Init},
		// Main sources and servers
		{"http", http.Init},     // rtsp source, HTTP server
		{"rtsp", rtsp.Init},     // rtsp source, RTSP server
		{"webrtc", webrtc.Init}, // webrtc source, WebRTC server
		// Main API
		{"mp4", mp4.Init},     // MP4 API
		{"hls", hls.Init},     // HLS API
		{"mjpeg", mjpeg.Init}, // MJPEG API
		// Other sources and servers
		{"hass", hass.Init},             // hass source, Hass API server
		{"homekit", homekit.Init},       // homekit source, HomeKit server
		{"onvif", onvif.Init},           // onvif source, ONVIF API server
		{"rtmp", rtmp.Init},             // rtmp source, RTMP server
		{"webtorrent", webtorrent.Init}, // webtorrent source, WebTorrent module
		{"wyoming", wyoming.Init},
		// Exec and script sources
		{"echo", echo.Init},
		{"exec", exec.Init},
		{"expr", expr.Init},
		{"ffmpeg", ffmpeg.Init},
		// Hardware sources
		{"alsa", alsa.Init},
		{"v4l2", v4l2.Init},
		// Other sources
		{"bubble", bubble.Init},
		{"doorbird", doorbird.Init},
		{"dvrip", dvrip.Init},
		{"eseecloud", eseecloud.Init},
		{"flussonic", flussonic.Init},
		{"gopro", gopro.Init},
		{"isapi", isapi.Init},
		{"ivideon", ivideon.Init},
		{"kasa", kasa.Init},
		{"mpegts", mpeg.Init},
		{"multitrans", multitrans.Init},
		{"nest", nest.Init},
		{"record", record.Init},
		{"ring", ring.Init},
		{"roborock", roborock.Init},
		{"tapo", tapo.Init},
		{"tuya", tuya.Init},
		{"wyze", wyze.Init},
		{"xiaomi", xiaomi.Init},
		{"yandex", yandex.Init},
		// Helper modules
		{"debug", debug.Init},
		{"ngrok", ngrok.Init},
		{"pinggy", pinggy.Init},
		{"srtp", srtp.Init},
	}

	for _, m := range modules {
		if app.Modules == nil || m.name == "" || slices.Contains(app.Modules, m.name) {
			m.init()
		}
	}

	shell.RunUntilSignal()
}
