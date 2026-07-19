package record

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/aac"
	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/h264"
	"github.com/AlexxIT/go2rtc/pkg/h265"
	"github.com/AlexxIT/go2rtc/pkg/mjpeg"
	"github.com/AlexxIT/go2rtc/pkg/mp4"
	"github.com/AlexxIT/go2rtc/pkg/pcm"
	"github.com/pion/rtp"
)

const maxKeyframeBytes = 2 * 1024 * 1024

var errRecorderBackpressure = errors.New("backpressure")

type Recorder struct {
	core.Connection

	src string

	mailbox *recordMailbox
	done    chan struct{}
	stop    sync.Once
	stopErr error

	prebufferNanos atomic.Int64
	snapshotNeeded atomic.Bool
	snapshotActive atomic.Bool
	accepting      atomic.Bool

	stateMu   sync.RWMutex
	recording bool
	fileName  string // constrained logical storage path
	startTime time.Time

	snapshotMu      sync.RWMutex
	lastKey         []byte
	lastKeyAt       time.Time
	snapshotCtx     context.Context
	snapshotCancel  context.CancelFunc
	failureMu       sync.RWMutex
	onFailure       func(*Recorder, error)
	failureNotified atomic.Bool

	// Everything below is owned exclusively by writerLoop after attachment.
	muxer            *mp4.Muxer
	file             *os.File
	writer           *bufio.Writer
	physicalName     string
	videoCodec       string
	startTS          map[byte]uint32
	prebufferPackets []*packetData
	prebufferBytes   int64
	prebufferStarted bool
	keyCollect       bool
	keyBuf           []byte
	keyBufBytes      int64
	thumbnailPending bool
}

func newRecorder(src string, prebuffer time.Duration) *Recorder {
	limits := currentLimits()
	snapshotCtx, snapshotCancel := context.WithCancel(context.Background())
	r := &Recorder{
		src:            src,
		mailbox:        newRecordMailbox(limits.writerQueueBytes()),
		done:           make(chan struct{}),
		muxer:          &mp4.Muxer{},
		startTS:        make(map[byte]uint32),
		snapshotCtx:    snapshotCtx,
		snapshotCancel: snapshotCancel,
	}
	r.SetPrebuffer(prebuffer)
	r.Connection = core.Connection{
		ID:         core.NewID(),
		FormatName: "record",
		Transport:  core.NewWriteBuffer(nil),
		Medias: []*core.Media{
			{
				Kind: core.KindVideo, Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{{Name: core.CodecH264}, {Name: core.CodecH265}, {Name: core.CodecJPEG}},
			},
			{
				Kind: core.KindAudio, Direction: core.DirectionSendonly,
				Codecs: []*core.Codec{
					{Name: core.CodecAAC}, {Name: core.CodecOpus}, {Name: core.CodecMP3},
					{Name: core.CodecPCMA}, {Name: core.CodecPCMU}, {Name: core.CodecPCM}, {Name: core.CodecPCML},
				},
			},
		},
	}
	go r.writerLoop()
	return r
}

func (r *Recorder) SetPrebuffer(prebuffer time.Duration) {
	if prebuffer < 0 {
		prebuffer = 0
	}
	r.prebufferNanos.Store(int64(prebuffer))
}

func (r *Recorder) SetSnapshotRequired(required bool) {
	r.snapshotNeeded.Store(required)
}

func (r *Recorder) SetFailureHandler(handler func(*Recorder, error)) {
	r.failureMu.Lock()
	r.onFailure = handler
	r.failureMu.Unlock()
}

func (r *Recorder) AddTrack(media *core.Media, _ *core.Codec, track *core.Receiver) error {
	trackID := byte(len(r.Senders))
	codec := track.Codec.Clone()
	handler := core.NewSender(media, codec)
	videoCodec := ""

	switch track.Codec.Name {
	case core.CodecH264:
		videoCodec = core.CodecH264
		handler.Handler = func(packet *rtp.Packet) {
			r.writeRTP(trackID, true, h264.IsKeyframe(packet.Payload), packet)
		}
		if track.Codec.IsRTP() {
			handler.Handler = h264.RTPDepay(track.Codec, handler.Handler)
		} else {
			handler.Handler = h264.RepairAVCC(track.Codec, handler.Handler)
		}
	case core.CodecH265:
		videoCodec = core.CodecH265
		handler.Handler = func(packet *rtp.Packet) {
			r.writeRTP(trackID, true, h265.IsKeyframe(packet.Payload), packet)
		}
		if track.Codec.IsRTP() {
			handler.Handler = h265.RTPDepay(track.Codec, handler.Handler)
		} else {
			handler.Handler = h265.RepairAVCC(track.Codec, handler.Handler)
		}
	case core.CodecJPEG:
		videoCodec = core.CodecJPEG
		var lastTime time.Time
		handler.Handler = func(packet *rtp.Packet) {
			now := time.Now()
			if now.Sub(lastTime) < 500*time.Millisecond {
				return
			}
			lastTime = now
			r.writeRTP(trackID, true, true, packet)
		}
		if track.Codec.IsRTP() {
			handler.Handler = mjpeg.RTPDepay(handler.Handler)
		}
	case core.CodecAAC, core.CodecOpus, core.CodecMP3:
		handler.Handler = func(packet *rtp.Packet) { r.writeRTP(trackID, false, false, packet) }
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
			r.writeRTP(trackID, false, false, packet)
		})
	default:
		return errors.New("unsupported codec: " + track.Codec.Name)
	}

	if err := r.addTrack(codec, videoCodec); err != nil {
		return err
	}
	handler.HandleRTP(track)
	r.Senders = append(r.Senders, handler)
	return nil
}

func (r *Recorder) addTrack(codec *core.Codec, videoCodec string) error {
	reply := make(chan error, 1)
	if !r.mailbox.pushControl(recordEvent{
		kind:       recordAddTrack,
		codec:      codec,
		videoCodec: videoCodec,
		reply:      reply,
	}) {
		return errors.New("recorder is stopped")
	}
	return <-reply
}

func (r *Recorder) writeRTP(trackID byte, video, keyframe bool, packet *rtp.Packet) {
	recording := r.isRecording()
	if !recording {
		if r.prebufferNanos.Load() <= 0 && (!video || !r.snapshotNeeded.Load()) {
			return
		}
		if r.prebufferNanos.Load() <= 0 && r.snapshotNeeded.Load() {
			if keyframe {
				r.snapshotActive.Store(true)
			}
			if !r.snapshotActive.Load() {
				return
			}
			if packet.Marker {
				r.snapshotActive.Store(false)
			}
		}
	} else if !r.accepting.Load() {
		return
	}

	size := packetSize(packet)
	if !recordMemory.reserve(size) {
		if recording {
			r.signalBackpressure()
		}
		return
	}

	// The clone happens at the ingress boundary. From this point the writer is
	// the only owner of the RTP packet and may normalize its AVCC payload.
	clone := packet.Clone()
	event := recordEvent{kind: recordPacket, packet: &packetData{
		trackID:  trackID,
		video:    video,
		keyframe: keyframe,
		packet:   clone,
		at:       time.Now(),
		size:     size,
	}}
	if !r.mailbox.pushPacket(event) {
		recordMemory.release(size)
		if recording {
			r.signalBackpressure()
		}
	}
}

func packetSize(packet *rtp.Packet) int64 {
	// rtp.Packet.Clone copies the header and payload. Account a small fixed
	// header allowance so the byte budget remains conservative.
	return int64(len(packet.Payload) + 64)
}

func (r *Recorder) StartRecording() error {
	return r.request(recordStart, "")
}

func (r *Recorder) StopRecording() error {
	return r.request(recordStop, "manual")
}

func (r *Recorder) request(kind recordEventKind, reason string) error {
	reply := make(chan error, 1)
	if !r.mailbox.pushControl(recordEvent{kind: kind, reason: reason, reply: reply}) {
		return errors.New("recorder is stopped")
	}
	return <-reply
}

func (r *Recorder) State() (bool, string, time.Time) {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return r.recording, r.fileName, r.startTime
}

func (r *Recorder) isRecording() bool {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	return r.recording
}

func (r *Recorder) LastKeyframe() ([]byte, time.Time, bool) {
	r.snapshotMu.RLock()
	defer r.snapshotMu.RUnlock()
	if len(r.lastKey) == 0 {
		return nil, time.Time{}, false
	}
	b := make([]byte, len(r.lastKey))
	copy(b, r.lastKey)
	return b, r.lastKeyAt, true
}

func (r *Recorder) snapshotContext() context.Context {
	return r.snapshotCtx
}

func (r *Recorder) signalBackpressure() {
	if r.accepting.CompareAndSwap(true, false) {
		r.mailbox.signalOverflow()
	}
}

func (r *Recorder) writerLoop() {
	defer close(r.done)
	defer r.mailbox.close()

	for {
		event, ok := r.mailbox.next()
		if !ok {
			return
		}
		switch event.kind {
		case recordPacket:
			r.handlePacket(event.packet)
		case recordAddTrack:
			r.muxer.AddTrack(event.codec)
			if event.videoCodec != "" {
				r.videoCodec = event.videoCodec
			}
			event.reply <- nil
		case recordStart:
			err := r.openSegment(time.Now())
			if err == nil {
				err = r.flushPrebuffer()
			}
			if err != nil {
				_ = r.closeSegment()
			} else {
				r.accepting.Store(true)
			}
			event.reply <- err
		case recordStop:
			err := r.closeSegment()
			event.reply <- err
		case recordOverflow:
			if r.isRecording() {
				_ = r.closeSegment()
				r.notifyFailure(errRecorderBackpressure)
			}
		case recordClose:
			err := r.closeSegment()
			r.clearPrebuffer()
			r.clearKeyBuffer()
			if event.reply != nil {
				event.reply <- err
			}
			return
		}
	}
}

func (r *Recorder) handlePacket(data *packetData) {
	if r.shouldCaptureSnapshot(data) {
		r.collectKeyframe(data)
	}
	if r.isRecording() {
		if err := r.writePacket(data); err != nil {
			recordMemory.release(data.size)
			_ = r.closeSegment()
			r.notifyFailure(err)
			return
		}
		recordMemory.release(data.size)
		return
	}
	r.appendPrebuffer(data)
}

func (r *Recorder) shouldCaptureSnapshot(data *packetData) bool {
	if !data.video {
		return false
	}
	if r.snapshotNeeded.Load() {
		return true
	}
	return r.isRecording() && !r.thumbnailPending
}

func (r *Recorder) collectKeyframe(data *packetData) {
	if data.keyframe {
		r.clearKeyBuffer()
		r.keyCollect = true
	}
	if !r.keyCollect {
		return
	}
	if len(r.keyBuf)+len(data.packet.Payload) > maxKeyframeBytes || !recordMemory.reserve(int64(len(data.packet.Payload))) {
		r.clearKeyBuffer()
		return
	}
	r.keyBuf = append(r.keyBuf, data.packet.Payload...)
	r.keyBufBytes += int64(len(data.packet.Payload))
	if !data.packet.Marker {
		return
	}

	thumbnail := ""
	if r.isRecording() && !r.thumbnailPending {
		thumbnail = r.physicalName
		r.thumbnailPending = true
	}
	task := snapshotTask{
		codec:     r.videoCodec,
		payload:   r.keyBuf,
		at:        data.at,
		bytes:     r.keyBufBytes,
		thumbnail: thumbnail,
	}
	r.keyBuf = nil
	r.keyBufBytes = 0
	r.keyCollect = false
	snapshots.submit(r, task)
}

func (r *Recorder) clearKeyBuffer() {
	recordMemory.release(r.keyBufBytes)
	r.keyBuf = nil
	r.keyBufBytes = 0
	r.keyCollect = false
}

func (r *Recorder) appendPrebuffer(data *packetData) {
	prebuffer := time.Duration(r.prebufferNanos.Load())
	if prebuffer <= 0 {
		recordMemory.release(data.size)
		return
	}
	if !r.prebufferStarted {
		if !data.video || !data.keyframe {
			recordMemory.release(data.size)
			return
		}
		r.prebufferStarted = true
	}
	r.prebufferPackets = append(r.prebufferPackets, data)
	r.prebufferBytes += data.size
	r.prunePrebuffer(data.at, prebuffer)
}

func (r *Recorder) prunePrebuffer(now time.Time, duration time.Duration) {
	limits := currentLimits()
	cutoff := now.Add(-duration)
	if len(r.prebufferPackets) == 0 {
		return
	}
	if !r.prebufferPackets[0].at.Before(cutoff) && r.prebufferBytes <= limits.prebufferBytes() {
		return
	}

	// Retain only a segment that begins at a recent keyframe and fits the
	// byte cap. If no such keyframe exists, retaining an undecodable history is
	// worse than waiting for the next IDR.
	remaining := r.prebufferBytes
	keep := -1
	for i, item := range r.prebufferPackets {
		if item.video && item.keyframe && !item.at.Before(cutoff) && remaining <= limits.prebufferBytes() {
			keep = i
			break
		}
		remaining -= item.size
	}
	if keep < 0 {
		r.clearPrebuffer()
		return
	}
	for _, item := range r.prebufferPackets[:keep] {
		recordMemory.release(item.size)
	}
	r.prebufferPackets = r.prebufferPackets[keep:]
	r.prebufferBytes = remaining
}

func (r *Recorder) clearPrebuffer() {
	for _, item := range r.prebufferPackets {
		recordMemory.release(item.size)
	}
	r.prebufferPackets = nil
	r.prebufferBytes = 0
	r.prebufferStarted = false
}

func (r *Recorder) flushPrebuffer() error {
	for i, item := range r.prebufferPackets {
		if err := r.writePacket(item); err != nil {
			recordMemory.release(item.size)
			for _, remaining := range r.prebufferPackets[i+1:] {
				recordMemory.release(remaining.size)
			}
			r.prebufferPackets = nil
			r.prebufferBytes = 0
			r.prebufferStarted = false
			return err
		}
		recordMemory.release(item.size)
	}
	r.prebufferPackets = nil
	r.prebufferBytes = 0
	r.prebufferStarted = false
	return nil
}

func (r *Recorder) openSegment(now time.Time) error {
	if r.isRecording() {
		return nil
	}
	base, _ := getDirAndRetention()
	ext := ".mp4"
	if r.videoCodec == core.CodecJPEG {
		ext = ".mjpeg"
	}
	file, physical, logical, err := createSegment(base, r.src, now, ext)
	if err != nil {
		return fmt.Errorf("create recording segment: %w", err)
	}

	r.file = file
	r.writer = bufio.NewWriterSize(file, 64*1024)
	r.physicalName = physical
	r.startTS = make(map[byte]uint32)
	r.thumbnailPending = false
	if r.videoCodec != core.CodecJPEG {
		r.muxer.Reset()
		initData, err := r.muxer.GetInit()
		if err != nil {
			_ = file.Close()
			r.file = nil
			r.writer = nil
			return err
		}
		if _, err = r.writer.Write(initData); err != nil {
			_ = file.Close()
			r.file = nil
			r.writer = nil
			return err
		}
	}
	r.stateMu.Lock()
	r.recording = true
	r.fileName = logical
	r.startTime = now
	r.stateMu.Unlock()
	return nil
}

func (r *Recorder) writePacket(data *packetData) error {
	if r.writer == nil {
		return errors.New("recording writer is unavailable")
	}
	packet := data.packet
	if data.video {
		for i := 0; i+4 < len(packet.Payload); {
			size := int(binary.BigEndian.Uint32(packet.Payload[i:]))
			if i+4+size > len(packet.Payload) {
				size = len(packet.Payload) - i - 4
				binary.BigEndian.PutUint32(packet.Payload[i:], uint32(size))
			}
			i += 4 + size
		}
	}
	if r.videoCodec == core.CodecJPEG {
		_, err := r.writer.Write(packet.Payload)
		return err
	}
	if ts, ok := r.startTS[data.trackID]; !ok {
		r.startTS[data.trackID] = packet.Timestamp
		packet.Timestamp = 0
	} else {
		packet.Timestamp -= ts
	}
	_, err := r.writer.Write(r.muxer.GetPayload(data.trackID, packet))
	return err
}

func (r *Recorder) closeSegment() error {
	r.accepting.Store(false)
	var err error
	if r.writer != nil {
		err = r.writer.Flush()
		r.writer = nil
	}
	if r.file != nil {
		if syncErr := r.file.Sync(); err == nil {
			err = syncErr
		}
		if closeErr := r.file.Close(); err == nil {
			err = closeErr
		}
		r.file = nil
	}
	r.stateMu.Lock()
	r.recording = false
	r.stateMu.Unlock()
	return err
}

func (r *Recorder) notifyFailure(err error) {
	if err == nil || !r.failureNotified.CompareAndSwap(false, true) {
		return
	}
	r.failureMu.RLock()
	handler := r.onFailure
	r.failureMu.RUnlock()
	if handler != nil {
		go handler(r, err)
	}
}

func (r *Recorder) Stop() error {
	r.stop.Do(func() {
		// Stop accepting before placing the close marker. The mailbox also gates
		// packet ingress with that marker, so packets accepted before it drain in
		// order and packets racing after it release their reservation immediately.
		r.accepting.Store(false)
		r.snapshotCancel()
		snapshots.cancel(r)
		reply := make(chan error, 1)
		if r.mailbox.pushControl(recordEvent{kind: recordClose, reply: reply}) {
			r.stopErr = <-reply
		}
		<-r.done
		if err := r.Connection.Stop(); r.stopErr == nil {
			r.stopErr = err
		}
	})
	return r.stopErr
}

type packetData struct {
	trackID  byte
	video    bool
	keyframe bool
	packet   *rtp.Packet
	at       time.Time
	size     int64
}

type recordEventKind uint8

const (
	recordPacket recordEventKind = iota
	recordAddTrack
	recordStart
	recordStop
	recordOverflow
	recordClose
)

type recordEvent struct {
	kind       recordEventKind
	packet     *packetData
	codec      *core.Codec
	videoCodec string
	reason     string
	reply      chan error
}

// recordMailbox is a byte-bounded FIFO. Packet producers never wait for disk
// I/O; control events share the same order so a start flushes exactly the
// packets received before it.
type recordMailbox struct {
	mu       sync.Mutex
	cond     *sync.Cond
	events   []recordEvent
	bytes    int64
	maxBytes int64
	overflow bool
	closing  bool
	closed   bool
}

func newRecordMailbox(maxBytes int64) *recordMailbox {
	m := &recordMailbox{maxBytes: maxBytes}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *recordMailbox) pushPacket(event recordEvent) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.closing || event.packet == nil || event.packet.size > m.maxBytes-m.bytes {
		return false
	}
	m.events = append(m.events, event)
	m.bytes += event.packet.size
	m.cond.Signal()
	return true
}

func (m *recordMailbox) pushControl(event recordEvent) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.closing {
		return false
	}
	if event.kind == recordClose {
		m.closing = true
	}
	m.events = append(m.events, event)
	m.cond.Signal()
	return true
}

func (m *recordMailbox) signalOverflow() {
	m.mu.Lock()
	if !m.closed && !m.closing {
		m.overflow = true
		m.cond.Signal()
	}
	m.mu.Unlock()
}

func (m *recordMailbox) next() (recordEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for len(m.events) == 0 && !m.overflow && !m.closed {
		m.cond.Wait()
	}
	if len(m.events) > 0 {
		event := m.events[0]
		m.events = m.events[1:]
		if event.packet != nil {
			m.bytes -= event.packet.size
		}
		return event, true
	}
	if m.overflow {
		m.overflow = false
		return recordEvent{kind: recordOverflow}, true
	}
	return recordEvent{}, false
}

func (m *recordMailbox) close() {
	m.mu.Lock()
	m.closed = true
	m.cond.Broadcast()
	m.mu.Unlock()
}
