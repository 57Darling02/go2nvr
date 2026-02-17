package record

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/57Darling02/go2nvr/internal/api"
	"github.com/57Darling02/go2nvr/internal/app"
	"github.com/57Darling02/go2nvr/internal/ffmpeg"
	"github.com/57Darling02/go2nvr/internal/streams"
	"github.com/57Darling02/go2nvr/pkg/aac"
	"github.com/57Darling02/go2nvr/pkg/core"
	"github.com/57Darling02/go2nvr/pkg/h264"
	"github.com/57Darling02/go2nvr/pkg/h264/annexb"
	"github.com/57Darling02/go2nvr/pkg/h265"
	"github.com/57Darling02/go2nvr/pkg/magic"
	"github.com/57Darling02/go2nvr/pkg/mjpeg"
	"github.com/57Darling02/go2nvr/pkg/mp4"
	"github.com/57Darling02/go2nvr/pkg/pcm"
	"github.com/pion/rtp"
	"github.com/rs/zerolog"
)

type recordRule struct {
	Src       string `yaml:"src"`
	Mode      string `yaml:"mode"`
	Segment   int    `yaml:"segment"`   // seconds
	Prebuffer int    `yaml:"prebuffer"` // seconds
	Post      int    `yaml:"post"`      // seconds
	Threshold int    `yaml:"threshold"`
}

var cfg struct {
	Mod struct {
		Dir       string       `yaml:"dir"`
		Retention int          `yaml:"retention"` // days, default 7
		Rules     []recordRule `yaml:"rules"`
	} `yaml:"record"`
}

var (
	log       zerolog.Logger
	recorders = make(map[string]*Recorder)
	mu        sync.Mutex
	confMu    sync.RWMutex
)

func Init() {
	log = app.GetLogger("record")

	cfg.Mod.Dir = "./records"
	cfg.Mod.Retention = 7 // Default 7 days
	app.LoadConfig(&cfg)

	if cfg.Mod.Dir == "" || cfg.Mod.Dir == "/" {
		cfg.Mod.Dir = "./records"
	}

	confMu.Lock()
	if err := os.MkdirAll(cfg.Mod.Dir, 0755); err != nil {
		log.Error().Err(err).Msg("[record] mkdir")
	}
	confMu.Unlock()

	// 启动磁盘清理协程
	go diskCleanup()

	// 启动配置中的流
	for _, rule := range cfg.Mod.Rules {
		if rule.Src != "" {
			// Copy rule to avoid loop variable issues
			r := rule
			start(r.Src, &r)
		}
	}

	api.HandleFunc("api/record", recordHandler)
	api.HandleFunc("api/record/file", fileHandler)
	api.HandleFunc("api/record/rules", rulesHandler)
	api.HandleFunc("api/record/config", configHandler)
}

func diskCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		confMu.RLock()
		retention := cfg.Mod.Retention
		baseDir := cfg.Mod.Dir
		confMu.RUnlock()

		if retention <= 0 {
			continue
		}
		cutoff := time.Now().AddDate(0, 0, -retention)
		log.Debug().Msgf("[record] cleaning up files older than %v", cutoff)

		files, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			// src/YYYY-MM-DD
			srcName := f.Name()
			srcDir := filepath.Join(baseDir, srcName)
			dateDirs, err := os.ReadDir(srcDir)
			if err != nil {
				continue
			}

			for _, d := range dateDirs {
				if !d.IsDir() {
					continue
				}
				t, err := time.Parse("2006-01-02", d.Name())
				if err == nil && t.Before(cutoff) {
					fullPath := filepath.Join(srcDir, d.Name())
					log.Info().Str("path", fullPath).Msg("[record] removing old recording")
					_ = os.RemoveAll(fullPath)
				}
			}
		}
	}
}

func start(src string, rule *recordRule) {
	name := src
	if i := strings.IndexByte(src, '?'); i > 0 {
		name = src[:i]
	}

	mu.Lock()
	defer mu.Unlock()

	if _, ok := recorders[name]; ok {
		return
	}

	rec := newRecorder(src, rule)
	recorders[name] = rec

	stream := streams.Get(name)
	if stream == nil {
		log.Warn().Str("src", name).Msg("[record] stream not found")
		delete(recorders, name)
		return
	}

	if err := stream.AddConsumer(rec); err != nil {
		log.Error().Err(err).Str("src", name).Msg("[record] add consumer")
		delete(recorders, name)
		return
	}

	// 初始化录制状态
	rec.mu.Lock()
	if rule != nil {
		rec.autoMode = rec.mode
		rec.autoOn = rec.autoMode == "always"
	} else {
		rec.manualOn = true // API 调用 Start 默认为手动开启
	}
	// 初始检查，如果模式是 always，立即开始
	rec.updateRecordingLocked(time.Now(), false)
	rec.mu.Unlock()
}

func Start(src string) { start(src, nil) }

func Stop(src string) {
	name := src
	if i := strings.IndexByte(src, '?'); i > 0 {
		name = src[:i]
	}

	mu.Lock()
	rec, ok := recorders[name]
	mu.Unlock()

	if !ok {
		return
	}

	rec.mu.Lock()
	rec.manualOn = false
	rec.updateRecordingLocked(time.Now(), false)
	keep := rec.autoMode != "" || rec.recording
	rec.mu.Unlock()

	if !keep {
		mu.Lock()
		delete(recorders, name)
		mu.Unlock()
		if stream := streams.Get(name); stream != nil {
			stream.RemoveConsumer(rec)
		} else {
			_ = rec.Stop()
		}
	}
}

type Recorder struct {
	core.Connection
	src string

	// Config
	segment   time.Duration
	mode      string
	prebuffer time.Duration
	post      time.Duration
	threshold int

	// Internal
	muxer     *mp4.Muxer
	file      *os.File
	writer    *bufio.Writer // Buffered I/O
	fileName  string
	buffer    []*packetData
	keyframes []int
	mu        sync.Mutex
	startTime time.Time

	// State
	recording bool
	manualOn  bool
	autoOn    bool
	autoMode  string

	// Motion
	lastFrame  []byte
	lastMotion time.Time
	videoTrack byte
	videoCodec string
	startTS    map[byte]uint32
	hasThumb   bool
}

type packetData struct {
	trackID byte
	packet  *rtp.Packet
	at      time.Time
}

func newRecorder(src string, rule *recordRule) *Recorder {
	// Defaults
	r := &Recorder{
		src:       src,
		segment:   600 * time.Second,
		mode:      "always",
		prebuffer: 8 * time.Second,
		post:      30 * time.Second,
		threshold: 5000,
		muxer:     &mp4.Muxer{},
		startTS:   make(map[byte]uint32),
	}

	// Merge Config
	if rule != nil {
		if rule.Mode != "" {
			r.mode = rule.Mode
		}
		if rule.Segment > 0 {
			r.segment = time.Duration(rule.Segment) * time.Second
		}
		if rule.Prebuffer > 0 {
			r.prebuffer = time.Duration(rule.Prebuffer) * time.Second
		}
		if rule.Post > 0 {
			r.post = time.Duration(rule.Post) * time.Second
		}
		if rule.Threshold > 0 {
			r.threshold = rule.Threshold
		}
	} else if i := strings.IndexByte(src, '?'); i > 0 {
		// Parse Query Params if no rule
		query := streams.ParseQuery(src[i+1:])
		if s := query.Get("mode"); s != "" {
			r.mode = s
		}
		if s := query.Get("segment"); s != "" {
			if d, _ := time.ParseDuration(s + "s"); d > 0 {
				r.segment = d
			}
		}
		if s := query.Get("prebuffer"); s != "" {
			if d, _ := time.ParseDuration(s + "s"); d > 0 {
				r.prebuffer = d
			}
		}
		if s := query.Get("post"); s != "" {
			if d, _ := time.ParseDuration(s + "s"); d > 0 {
				r.post = d
			}
		}
		if s := query.Get("threshold"); s != "" {
			if v := core.Atoi(s); v > 0 {
				r.threshold = v
			}
		}
		r.src = src[:i]
	}

	if rule != nil {
		r.autoMode = r.mode
	}

	// Core Connection Setup
	r.Connection = core.Connection{
		ID:         core.NewID(),
		FormatName: "record",
		Transport:  core.NewWriteBuffer(nil),
		Medias: []*core.Media{
			{
				Kind: core.KindVideo, Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{{Name: core.CodecH264}, {Name: core.CodecH265}},
			},
			{
				Kind: core.KindAudio, Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{
					{Name: core.CodecAAC}, {Name: core.CodecOpus}, {Name: core.CodecMP3},
					{Name: core.CodecPCMA}, {Name: core.CodecPCMU}, {Name: core.CodecPCM},
					{Name: core.CodecPCML}, // Added CodecPCML to match original
				},
			},
		},
	}
	return r
}

func (r *Recorder) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	trackID := byte(len(r.Senders))
	codec := track.Codec.Clone()
	handler := core.NewSender(media, codec)

	// Codec Handlers
	switch track.Codec.Name {
	case core.CodecH264:
		r.videoTrack = trackID
		r.videoCodec = core.CodecH264
		handler.Handler = func(packet *rtp.Packet) {
			r.writeRTP(trackID, packet, h264.IsKeyframe(packet.Payload))
		}
		if track.Codec.IsRTP() {
			handler.Handler = h264.RTPDepay(track.Codec, handler.Handler)
		} else {
			handler.Handler = h264.RepairAVCC(track.Codec, handler.Handler)
		}
	case core.CodecH265:
		r.videoTrack = trackID
		r.videoCodec = core.CodecH265
		handler.Handler = func(packet *rtp.Packet) {
			r.writeRTP(trackID, packet, h265.IsKeyframe(packet.Payload))
		}
		if track.Codec.IsRTP() {
			handler.Handler = h265.RTPDepay(track.Codec, handler.Handler)
		} else {
			handler.Handler = h265.RepairAVCC(track.Codec, handler.Handler)
		}
	case core.CodecJPEG:
		r.videoTrack = trackID
		r.videoCodec = core.CodecJPEG
		var lastTime time.Time
		handler.Handler = func(packet *rtp.Packet) {
			now := time.Now()
			if now.Sub(lastTime) < 500*time.Millisecond {
				return
			}
			lastTime = now
			r.writeRTP(trackID, packet, true)
		}
		if track.Codec.IsRTP() {
			handler.Handler = mjpeg.RTPDepay(handler.Handler)
		}
	case core.CodecAAC, core.CodecOpus, core.CodecMP3:
		handler.Handler = func(packet *rtp.Packet) { r.writeRTP(trackID, packet, false) }
		if track.Codec.Name == core.CodecAAC && track.Codec.IsRTP() {
			handler.Handler = aac.RTPDepay(handler.Handler)
		}
	case core.CodecPCMA, core.CodecPCMU, core.CodecPCM, core.CodecPCML:
		codec.Name = core.CodecFLAC
		if codec.Channels == 2 {
			codec.Channels = 1
			codec.ClockRate *= 2
		}
		handler.Handler = pcm.FLACEncoder(track.Codec.Name, codec.ClockRate, func(packet *rtp.Packet) {
			r.writeRTP(trackID, packet, false)
		})
	default:
		return errors.New("unsupported codec: " + track.Codec.Name)
	}

	r.muxer.AddTrack(codec)
	handler.HandleRTP(track)
	r.Senders = append(r.Senders, handler)
	return nil
}

func (r *Recorder) updateRecordingLocked(now time.Time, isKeyframe bool) {
	// 逻辑：如果是关键帧，检查是否需要开启或切片；如果不是，检查是否需要停止
	if !isKeyframe {
		if !(r.manualOn || r.autoOn) && r.recording {
			r.closeFile()
		}
		return
	}

	if r.manualOn || r.autoOn {
		// 需要录制
		if !r.recording {
			r.pruneBuffer(now)      // 开始录制前清理不需要的缓冲
			r.rotateFile(now, true) // 开启新文件
		} else if time.Since(r.startTime) >= r.segment {
			r.rotateFile(now, false) // 达到分段时长，切片
		}
	} else if r.recording {
		r.closeFile()
	}
}

func (r *Recorder) writeRTP(trackID byte, packet *rtp.Packet, isKeyframe bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// 运动检测 (仅在 Keyframe 且 motion 模式下)
	if isKeyframe && r.autoMode == "motion" {
		if r.checkMotion(packet.Payload) {
			r.autoOn = true
			r.lastMotion = now
		} else if r.autoOn && now.Sub(r.lastMotion) > r.post {
			r.autoOn = false
		}
	}

	r.updateRecordingLocked(now, isKeyframe)

	if r.recording {
		// Flush buffer if exists
		if len(r.buffer) > 0 {
			startIdx := 0
			if len(r.keyframes) > 0 {
				startIdx = r.keyframes[0] // 从第一个关键帧开始写
			}
			for _, item := range r.buffer[startIdx:] {
				r.writeToFile(item.trackID, item.packet)
			}
			r.buffer = nil
			r.keyframes = nil
		}
		r.writeToFile(trackID, packet)
	} else {
		// Buffering
		// Clone packet to avoid memory reuse issues
		clone := packet.Clone()
		r.buffer = append(r.buffer, &packetData{trackID: trackID, packet: clone, at: now})
		if isKeyframe {
			r.keyframes = append(r.keyframes, len(r.buffer)-1)
		}
		// 定期清理过多的缓冲，防止内存溢出 (每100个包清理一次)
		if len(r.buffer)%100 == 0 {
			r.pruneBuffer(now)
		}
	}
}

// 优化后的 Buffer 清理
func (r *Recorder) pruneBuffer(now time.Time) {
	if len(r.buffer) == 0 {
		return
	}
	cut := now.Add(-r.prebuffer)
	// 找到第一个保留帧的索引
	keepIdx := 0
	// 确保不删掉最新的关键帧之前的数据
	limitIdx := len(r.buffer)
	if len(r.keyframes) > 0 {
		limitIdx = r.keyframes[len(r.keyframes)-1]
	}

	for keepIdx < len(r.buffer) && keepIdx < limitIdx && r.buffer[keepIdx].at.Before(cut) {
		keepIdx++
	}

	if keepIdx > 0 {
		// 重新切片
		r.buffer = r.buffer[keepIdx:]
		// 重建 keyframes 索引
		var newKeys []int
		for _, k := range r.keyframes {
			if k >= keepIdx {
				newKeys = append(newKeys, k-keepIdx)
			}
		}
		r.keyframes = newKeys
	}
}

// rotateFile 实现无缝分段和新文件创建
// isStart: true 表示是从停止状态开始录制，false 表示分段切换
func (r *Recorder) rotateFile(now time.Time, isStart bool) {
	confMu.RLock()
	baseDir := cfg.Mod.Dir
	confMu.RUnlock()

	dir := filepath.Join(baseDir, r.src, now.Format("2006-01-02"))
	_ = os.MkdirAll(dir, 0755)

	// 使用毫秒后缀防止文件名冲突
	ext := ".mp4"
	if r.videoCodec == core.CodecJPEG {
		ext = ".mjpeg"
	}
	filename := filepath.Join(dir, now.Format("15-04-05.000")+ext)
	f, err := os.Create(filename)
	if err != nil {
		log.Error().Err(err).Str("src", r.src).Msg("[record] create file failed")
		// 如果创建失败，如果是切片，则继续用旧文件；如果是新开始，则放弃
		if isStart {
			r.recording = false
		}
		return
	}

	// 1. 如果有旧文件，先 Flush 数据到磁盘
	if r.writer != nil {
		_ = r.writer.Flush()
	}

	// 2. 保存旧文件句柄以便异步关闭
	oldFile := r.file

	// 3. 切换到新文件
	r.file = f
	r.writer = bufio.NewWriterSize(f, 64*1024) // 64KB Buffer
	r.fileName = filename
	r.startTime = now
	r.recording = true
	r.lastMotion = now
	r.startTS = make(map[byte]uint32) // 重置时间戳映射
	r.hasThumb = false


	// 4. 写入 MP4 头部
	if r.videoCodec != core.CodecJPEG {
		r.muxer.Reset()
		initData, _ := r.muxer.GetInit()
		_, _ = r.writer.Write(initData)
	}

	log.Info().Str("src", r.src).Str("file", filename).Msg("[record] rotate/start file")

	// 5. 异步关闭旧文件，避免阻塞 RTP 循环
	if oldFile != nil {
		go func(f *os.File) {
			_ = f.Close()
		}(oldFile)
	}
}

func (r *Recorder) writeToFile(trackID byte, packet *rtp.Packet) {
	if r.writer == nil {
		return
	}

	// 时间戳重写
	clone := *packet // Shallow copy is fine here as we only modify struct fields

	// Create a deep copy of payload if we need to modify it
	// Only needed if we are actually writing AVCC size header
	if trackID == r.videoTrack {
		payload := make([]byte, len(packet.Payload))
		copy(payload, packet.Payload)
		clone.Payload = payload

		for i := 0; i+4 < len(clone.Payload); {
			size := int(binary.BigEndian.Uint32(clone.Payload[i:]))
			if i+4+size > len(clone.Payload) {
				size = len(clone.Payload) - i - 4
				binary.BigEndian.PutUint32(clone.Payload[i:], uint32(size))
			}
			i += 4 + size
		}
		if !r.hasThumb {
			r.hasThumb = true
			go r.saveThumbnail()
		}
	}

	if r.videoCodec == core.CodecJPEG {
		if _, err := r.writer.Write(packet.Payload); err != nil {
			log.Error().Err(err).Str("src", r.src).Msg("[record] write mjpeg error")
			r.closeFile()
		}
		return
	}

	if ts, ok := r.startTS[trackID]; !ok {
		r.startTS[trackID] = clone.Timestamp
		clone.Timestamp = 0
	} else {
		clone.Timestamp -= ts
	}

	b := r.muxer.GetPayload(trackID, &clone)
	if _, err := r.writer.Write(b); err != nil {
		log.Error().Err(err).Str("src", r.src).Msg("[record] write error")
		r.closeFile()
	}
}

func (r *Recorder) closeFile() {
	if r.writer != nil {
		_ = r.writer.Flush()
		r.writer = nil
	}
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
		log.Info().Str("src", r.src).Msg("[record] stop recording")
	}
	r.recording = false
}

func (r *Recorder) checkMotion(payload []byte) bool {
	var b []byte
	format := "h264"

	switch r.videoCodec {
	case core.CodecH265:
		format = "hevc"
		b = annexb.DecodeAVCC(payload, true)
	case core.CodecJPEG:
		format = "mjpeg"
		b = payload
	default:
		b = annexb.DecodeAVCC(payload, true)
	}

	// 使用 Context 防止 ffmpeg 僵死
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "quiet",
		"-f", format, "-i", "pipe:0",
		"-vf", "scale=32:32,format=gray",
		"-f", "rawvideo", "pipe:1")

	cmd.Stdin = bytes.NewReader(b)
	out, err := cmd.Output()
	if err != nil {
		// 超时或错误，忽略本次检测
		return false
	}

	if len(out) != 32*32 {
		return false
	}

	changed := false
	if r.lastFrame != nil {
		diff := 0
		for i := 0; i < len(out); i++ {
			d := int(out[i]) - int(r.lastFrame[i])
			if d < 0 {
				d = -d
			}
			diff += d
		}
		if diff > r.threshold {
			changed = true
		}
	}
	r.lastFrame = out
	return changed
}

func (r *Recorder) saveThumbnail() {
	filename := r.fileName
	stream := streams.Get(r.src)
	if stream == nil {
		return
	}

	cons := magic.NewKeyframe()
	if err := stream.AddConsumer(cons); err != nil {
		log.Warn().Err(err).Str("src", r.src).Msg("[record] add keyframe consumer failed")
		return
	}

	once := &core.OnceBuffer{}
	_, _ = cons.WriteTo(once)
	stream.RemoveConsumer(cons)

	b := once.Buffer()
	if len(b) == 0 {
		return
	}

	var (
		jpegData []byte
		err      error
	)

	switch cons.CodecName() {
	case core.CodecH264, core.CodecH265:
		jpegData, err = ffmpeg.JPEGWithScale(b, 640, -1)
	case core.CodecJPEG:
		jpegData = mjpeg.FixJPEG(b)
	default:
		return
	}

	if err != nil {
		log.Warn().Err(err).Str("src", r.src).Msg("[record] save thumbnail failed")
		return
	}

	thumbName := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".thumb"
	if err := os.WriteFile(thumbName, jpegData, 0644); err != nil {
		log.Warn().Err(err).Str("src", r.src).Msg("[record] create thumbnail file failed")
	}
}

func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manualOn = false
	r.autoOn = false
	r.closeFile()
	return r.Connection.Stop()
}

func recordHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("path") != "" {
		listItems(w, r)
		return
	}

	src := r.URL.Query().Get("src")

	// 1. 查询所有流的状态
	if src == "" {
		allStreams := streams.GetAllNames()
		states := make([]map[string]interface{}, 0, len(allStreams))

		mu.Lock()
		// 创建临时映射以快速查找 recorder
		recMap := make(map[string]*Recorder, len(recorders))
		for name, rec := range recorders {
			recMap[name] = rec
		}
		mu.Unlock()

		for _, name := range allStreams {
			state := map[string]interface{}{
				"name":   name,
				"status": "stopped",
			}

			if rec, ok := recMap[name]; ok {
				rec.mu.Lock()
				if rec.recording {
					state["status"] = "recording"
					state["file"] = rec.fileName
					state["duration"] = time.Since(rec.startTime).String()
				} else if rec.autoMode != "" {
					state["status"] = "idle"
					state["mode"] = rec.autoMode
				}

				state["manual"] = rec.manualOn
				state["auto_active"] = rec.autoOn

				rec.mu.Unlock()
			}
			states = append(states, state)
		}

		api.ResponseJSON(w, states)
		return
	}

	// 2. 查询单个流的具体状态 (Stream 视角)
	// 即使流没配置录制，我们也返回一个默认的 status，而不是 404
	state := map[string]interface{}{
		"status": "stopped", // 默认状态
	}

	mu.Lock()
	rec, ok := recorders[src]
	mu.Unlock()

	if ok {
		rec.mu.Lock()
		if rec.recording {
			state["status"] = "recording"
			state["file"] = rec.fileName // 顺便返回当前文件名，方便调试
			state["duration"] = time.Since(rec.startTime).String()
		} else if rec.autoMode != "" {
			state["status"] = "idle"
			state["mode"] = rec.autoMode
		}
		// 如果既没录，又是 manual 模式且 manualOn=false，那就是 stopped

		state["manual"] = rec.manualOn
		state["auto_active"] = rec.autoOn

		rec.mu.Unlock()
	}

	// 处理 POST 请求（手动开关）
	if r.Method == "POST" {
		action := r.URL.Query().Get("action") // start / stop
		switch action {
		case "start":
			Start(src)                    // 即使没有 recorder 也会创建一个
			state["status"] = "recording" // 乐观更新状态
		case "stop":
			Stop(src)
			state["status"] = "stopped"
		}
	}

	api.ResponseJSON(w, state)
}

func listItems(w http.ResponseWriter, r *http.Request) {
	confMu.RLock()
	baseDir := cfg.Mod.Dir
	confMu.RUnlock()

	pathRel := r.URL.Query().Get("path")
	fullPath := filepath.Join(baseDir, filepath.FromSlash(pathRel))

	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(baseDir)) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type Item struct {
		Name    string `json:"name"`
		IsFile  bool   `json:"is_file"`
		Size    int64  `json:"size,omitempty"`
		ModTime int64  `json:"mod_time,omitempty"`
	}

	var items []Item
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		item := Item{
			Name:   entry.Name(),
			IsFile: !entry.IsDir(),
		}

		if item.IsFile {
			ext := filepath.Ext(item.Name)
			if ext != ".mp4" && ext != ".mjpeg" {
				continue
			}
			item.Size = info.Size()
			item.ModTime = info.ModTime().Unix()
		}

		items = append(items, item)
	}

	api.ResponseJSON(w, items)
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	pathRel := r.URL.Query().Get("path")
	if pathRel == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	confMu.RLock()
	baseDir := cfg.Mod.Dir
	confMu.RUnlock()

	fullPath := filepath.Join(baseDir, filepath.FromSlash(pathRel))
	if !strings.HasPrefix(fullPath, filepath.Clean(baseDir)) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	switch r.Method {
	case "GET":
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(fullPath))
		}
		http.ServeFile(w, r, fullPath)

	case "DELETE":
		if err := os.Remove(fullPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
