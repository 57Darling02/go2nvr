package record

import (
	"bufio"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
)

type Recorder struct {
	core.Connection
	src string

	prebuffer time.Duration

	muxer     *mp4.Muxer
	file      *os.File
	writer    *bufio.Writer
	fileName  string
	buffer    []*packetData
	keyframes []int
	mu        sync.Mutex
	startTime time.Time

	recording  bool
	videoTrack byte
	videoCodec string
	startTS    map[byte]uint32
	hasThumb   bool
	lastKeyAt  time.Time
	lastKey    []byte
	keyCollect bool
	keyBuf     []byte
}

type packetData struct {
	trackID byte
	packet  *rtp.Packet
	at      time.Time
}

func newRecorder(src string, prebuffer time.Duration) *Recorder {
	r := &Recorder{
		src:       src,
		prebuffer: prebuffer,
		muxer:     &mp4.Muxer{},
		startTS:   make(map[byte]uint32),
	}

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
					{Name: core.CodecPCML},
				},
			},
		},
	}
	return r
}

func (r *Recorder) SetPrebuffer(prebuffer time.Duration) {
	r.mu.Lock()
	r.prebuffer = prebuffer
	r.mu.Unlock()
}

func (r *Recorder) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	trackID := byte(len(r.Senders))
	codec := track.Codec.Clone()
	handler := core.NewSender(media, codec)

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

func (r *Recorder) writeRTP(trackID byte, packet *rtp.Packet, isKeyframe bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if trackID == r.videoTrack {
		if isKeyframe {
			r.keyCollect = true
			r.keyBuf = r.keyBuf[:0]
		}
		if r.keyCollect {
			const maxKeyBytes = 2 * 1024 * 1024
			if len(r.keyBuf)+len(packet.Payload) <= maxKeyBytes {
				r.keyBuf = append(r.keyBuf, packet.Payload...)
			}
			if packet.Marker {
				if len(r.keyBuf) > 0 {
					var jpegData []byte
					switch r.videoCodec {
					case core.CodecH264:
						jpegData, _ = ffmpeg.JPEGWithScale(annexb.DecodeAVCC(r.keyBuf, true), 640, -1)
					case core.CodecH265:
						jpegData, _ = ffmpeg.JPEGWithScale(annexb.DecodeAVCC(r.keyBuf, true), 640, -1)
					case core.CodecJPEG:
						jpegData = mjpeg.FixJPEG(r.keyBuf)
					}

					if len(jpegData) > 0 {
						r.lastKey = jpegData
						r.lastKeyAt = now
					}
				}
				r.keyCollect = false
			}
		}
	}

	if r.recording {
		if len(r.buffer) > 0 {
			r.flushBufferToFileLocked()
		}
		r.writeToFile(trackID, packet)
		return
	}

	clone := packet.Clone()
	r.buffer = append(r.buffer, &packetData{trackID: trackID, packet: clone, at: now})
	if isKeyframe {
		r.keyframes = append(r.keyframes, len(r.buffer)-1)
	}
	r.pruneBuffer(now)
}

func (r *Recorder) LastKeyframe() ([]byte, time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.lastKey) == 0 {
		return nil, time.Time{}, false
	}
	b := make([]byte, len(r.lastKey))
	copy(b, r.lastKey)
	return b, r.lastKeyAt, true
}

func (r *Recorder) StartRecording() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return
	}

	now := time.Now()
	r.pruneBuffer(now)
	r.openFile(now)
	if r.recording && len(r.buffer) > 0 {
		r.flushBufferToFileLocked()
	}
}

func (r *Recorder) StopRecording() {
	r.mu.Lock()
	r.closeFile()
	r.mu.Unlock()
}

func (r *Recorder) State() (bool, string, time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recording, r.fileName, r.startTime
}

func (r *Recorder) pruneBuffer(now time.Time) {
	if len(r.buffer) == 0 {
		return
	}
	if r.prebuffer <= 0 {
		r.buffer = nil
		r.keyframes = nil
		return
	}

	cut := now.Add(-r.prebuffer)
	keepIdx := 0
	limitIdx := len(r.buffer)
	if len(r.keyframes) > 0 {
		limitIdx = r.keyframes[len(r.keyframes)-1]
	}

	for keepIdx < len(r.buffer) && keepIdx < limitIdx && r.buffer[keepIdx].at.Before(cut) {
		keepIdx++
	}

	if keepIdx > 0 {
		r.buffer = r.buffer[keepIdx:]
		var newKeys []int
		for _, k := range r.keyframes {
			if k >= keepIdx {
				newKeys = append(newKeys, k-keepIdx)
			}
		}
		r.keyframes = newKeys
	}
}

func (r *Recorder) flushBufferToFileLocked() {
	startIdx := 0
	if len(r.keyframes) > 0 {
		startIdx = r.keyframes[0]
	}
	for _, item := range r.buffer[startIdx:] {
		r.writeToFile(item.trackID, item.packet)
	}
	r.buffer = nil
	r.keyframes = nil
}

func (r *Recorder) openFile(now time.Time) {
	baseDir, _ := getDirAndRetention()
	dir := filepath.Join(baseDir, r.src, now.Format("2006-01-02"))
	_ = os.MkdirAll(dir, 0755)

	ext := ".mp4"
	if r.videoCodec == core.CodecJPEG {
		ext = ".mjpeg"
	}
	filename := filepath.Join(dir, now.Format("15-04-05")+ext)
	f, err := os.Create(filename)
	if err != nil {
		log.Error().Err(err).Str("src", r.src).Msg("[record] create file failed")
		r.recording = false
		return
	}

	r.file = f
	r.writer = bufio.NewWriterSize(f, 64*1024)
	r.fileName = filename
	r.startTime = now
	r.recording = true
	r.startTS = make(map[byte]uint32)
	r.hasThumb = false

	if r.videoCodec != core.CodecJPEG {
		r.muxer.Reset()
		initData, _ := r.muxer.GetInit()
		_, _ = r.writer.Write(initData)
	}
}

func (r *Recorder) writeToFile(trackID byte, packet *rtp.Packet) {
	if r.writer == nil {
		return
	}

	clone := *packet
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
	}
	r.recording = false
}

func (r *Recorder) saveThumbnail() {
	filename := r.fileName
	if b, _, ok := r.LastKeyframe(); ok && len(b) > 0 {
		thumbName := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".thumb"
		_ = os.WriteFile(thumbName, b, 0644)
		return
	}

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
	_ = os.WriteFile(thumbName, jpegData, 0644)
}

func (r *Recorder) Stop() error {
	r.StopRecording()
	return r.Connection.Stop()
}
