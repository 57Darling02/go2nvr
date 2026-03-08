package ttspush

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlexxIT/go2rtc/internal/api"
	"github.com/AlexxIT/go2rtc/internal/ttspush/target"
)

type Request struct {
	Text           string `json:"text"`
	Voice          string `json:"voice"`
	Rate           string `json:"rate"`
	Pitch          string `json:"pitch"`
	Volume         string `json:"volume"`
	Format         string `json:"format"`
	Proxy          string `json:"proxy"`
	ConnectTimeout int    `json:"connect_timeout"`
	ReceiveTimeout int    `json:"receive_timeout"`

	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	URL        string `json:"url"`
	Dst        string `json:"dst"`

	SampleRate  int  `json:"sample_rate"`
	ChunkMS     int  `json:"chunk_ms"`
	Realtime    bool `json:"realtime"`
	InsecureTLS bool `json:"insecure_tls"`
}

type Response struct {
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	Format     string `json:"format"`
	Bytes      int    `json:"bytes"`
	Voice      string `json:"voice"`
	Engine     string `json:"engine"`
}

func Init() {
	api.HandleFunc("api/ttspush/push", apiPush)
}

func apiPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	req, err := parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(req.ReceiveTimeout+30)*time.Second)
	defer cancel()

	genReq := GenerateRequest{
		Text:           req.Text,
		Voice:          req.Voice,
		Rate:           req.Rate,
		Pitch:          req.Pitch,
		Volume:         req.Volume,
		Proxy:          req.Proxy,
		ConnectTimeout: req.ConnectTimeout,
		ReceiveTimeout: req.ReceiveTimeout,
		SampleRate:     req.SampleRate,
	}

	var (
		sentBytes int
		dst       string
		format    string
	)

	switch req.TargetType {
	case "ipwebcam":
		genReq.Format = "pcm16le"
		sentBytes, err = target.PushIPWebcam(
			ctx,
			target.IPWebcamOptions{
				URL:         req.URL,
				SampleRate:  req.SampleRate,
				ChunkMS:     req.ChunkMS,
				Realtime:    req.Realtime,
				InsecureTLS: req.InsecureTLS,
			},
			func(ctx context.Context) (io.ReadCloser, error) {
				return GenerateStream(ctx, genReq)
			},
		)
		dst = req.URL
		format = "pcm16le"
	case "backchannel":
		genReq.Format = req.Format
		sentBytes, err = target.PushBackchannel(
			ctx,
			target.BackchannelOptions{Dst: req.Dst},
			func(ctx context.Context) (io.ReadCloser, error) {
				return GenerateStream(ctx, genReq)
			},
		)
		dst = req.Dst
		format = req.Format
	default:
		http.Error(w, "unsupported target_type", http.StatusBadRequest)
		return
	}

	if err != nil {
		code := http.StatusBadGateway
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			code = http.StatusGatewayTimeout
		case strings.Contains(err.Error(), "queue is full"), strings.Contains(err.Error(), "client is closed"):
			code = http.StatusServiceUnavailable
		}
		http.Error(w, err.Error(), code)
		return
	}

	api.ResponseJSON(w, &Response{
		TargetType: req.TargetType,
		Target:     dst,
		Format:     format,
		Bytes:      sentBytes,
		Voice:      req.Voice,
		Engine:     "edge-tts-go",
	})
}

func parseRequest(r *http.Request) (*Request, error) {
	req := &Request{
		Voice:          defaultVoice,
		Rate:           defaultRate,
		Pitch:          defaultPitch,
		Volume:         defaultVolume,
		Proxy:          os.Getenv("HTTPS_PROXY"),
		ConnectTimeout: defaultConnectTimeout,
		ReceiveTimeout: defaultReceiveTimeout,
		SampleRate:     defaultSampleRate,
		ChunkMS:        defaultChunkMS,
		Realtime:       defaultRealtime,
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			return nil, err
		}
	}

	q := r.URL.Query()
	if v := q.Get("text"); v != "" {
		req.Text = v
	}
	if v := q.Get("voice"); v != "" {
		req.Voice = v
	}
	if v := q.Get("rate"); v != "" {
		req.Rate = v
	}
	if v := q.Get("pitch"); v != "" {
		req.Pitch = v
	}
	if v := q.Get("volume"); v != "" {
		req.Volume = v
	}
	if v := q.Get("format"); v != "" {
		req.Format = v
	}
	if v := q.Get("proxy"); v != "" {
		req.Proxy = v
	}
	if v := q.Get("connect_timeout"); v != "" {
		req.ConnectTimeout = toInt(v, req.ConnectTimeout)
	}
	if v := q.Get("receive_timeout"); v != "" {
		req.ReceiveTimeout = toInt(v, req.ReceiveTimeout)
	}
	if v := q.Get("target_type"); v != "" {
		req.TargetType = v
	}
	if v := q.Get("target"); v != "" {
		req.Target = v
	}
	if v := q.Get("url"); v != "" {
		req.URL = v
	}
	if v := q.Get("dst"); v != "" {
		req.Dst = v
	}
	if v := q.Get("sample_rate"); v != "" {
		req.SampleRate = toInt(v, req.SampleRate)
	}
	if v := q.Get("chunk_ms"); v != "" {
		req.ChunkMS = toInt(v, req.ChunkMS)
	}
	if v := q.Get("realtime"); v != "" {
		req.Realtime = toBool(v, req.Realtime)
	}
	if v := q.Get("insecure_tls"); v != "" {
		req.InsecureTLS = toBool(v, req.InsecureTLS)
	}

	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		return nil, errors.New("text is required")
	}

	req.TargetType = strings.ToLower(strings.TrimSpace(req.TargetType))
	req.Target = strings.TrimSpace(req.Target)
	req.URL = strings.TrimSpace(req.URL)
	req.Dst = strings.TrimSpace(req.Dst)

	if req.TargetType == "" {
		switch {
		case req.URL != "", strings.HasPrefix(req.Target, "ws://"), strings.HasPrefix(req.Target, "wss://"):
			req.TargetType = "ipwebcam"
		case req.Dst != "", req.Target != "":
			req.TargetType = "backchannel"
		default:
			return nil, errors.New("target_type is required")
		}
	}

	switch req.TargetType {
	case "ipwebcam", "wss":
		req.TargetType = "ipwebcam"
		if req.URL == "" {
			req.URL = req.Target
		}
		if req.URL == "" {
			return nil, errors.New("url is required for ipwebcam target")
		}
	case "backchannel", "stream":
		req.TargetType = "backchannel"
		if req.Dst == "" {
			req.Dst = req.Target
		}
		if req.Dst == "" {
			return nil, errors.New("dst is required for backchannel target")
		}
		req.Format = strings.ToLower(strings.TrimSpace(req.Format))
		if req.Format == "" {
			req.Format = defaultBackchannelFormat
		}
		if req.Format != defaultBackchannelFormat {
			return nil, errors.New("backchannel currently supports format=wav only")
		}
	default:
		return nil, errors.New("unsupported target_type: " + req.TargetType)
	}

	return req, nil
}

func toInt(s string, fallback int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return fallback
}

func toBool(s string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
