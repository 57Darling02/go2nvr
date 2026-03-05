package trigger

import (
	"bytes"
	"image/jpeg"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func SetLogger(l zerolog.Logger) {
	log = l
}

type Frame struct {
	JPEG       []byte
	JPEGWidth  int
	JPEGHeight int
	Gray       []byte
	Width      int
	Height     int
	At         time.Time
}

type RawFrame struct {
	Payload []byte
	At      time.Time
}

type Rule struct {
	Src       string
	Enabled   bool
	TypeID    int
	Threshold int
	Post      time.Duration
	Interval  time.Duration
}

type Detector interface {
	Detect(prev, cur Frame) bool
}

type DetectorFactory func(rule Rule) Detector

type GetFrameFunc func(src string) (RawFrame, bool)
type StartFunc func(src string)
type StopFunc func(src string)
type IsRecordingFunc func(src string) bool

type Manager struct {
	getFrame    GetFrameFunc
	start       StartFunc
	stop        StopFunc
	isRecording IsRecordingFunc

	mu      sync.Mutex
	workers map[string]*worker
}

type worker struct {
	rule     Rule
	detector Detector
	stopCh   chan struct{}
}

type DetectorInfo struct {
	ID   int    `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type registeredDetector struct {
	info    DetectorInfo
	factory DetectorFactory
}

var (
	regMu         sync.RWMutex
	registryByKey = map[string]registeredDetector{}
	registryByID  = map[int]registeredDetector{}
)

func Register(id int, key, name string, factory DetectorFactory) {
	if key == "" || factory == nil {
		return
	}
	d := registeredDetector{
		info: DetectorInfo{
			ID:   id,
			Key:  key,
			Name: name,
		},
		factory: factory,
	}
	regMu.Lock()
	registryByKey[key] = d
	if id > 0 {
		registryByID[id] = d
	}
	regMu.Unlock()
}

func ListDetectors() []DetectorInfo {
	regMu.RLock()
	byKey := make([]registeredDetector, 0, len(registryByKey))
	for _, d := range registryByKey {
		byKey = append(byKey, d)
	}
	regMu.RUnlock()

	sort.Slice(byKey, func(i, j int) bool {
		if byKey[i].info.ID == byKey[j].info.ID {
			return byKey[i].info.Key < byKey[j].info.Key
		}
		if byKey[i].info.ID == 0 {
			return false
		}
		if byKey[j].info.ID == 0 {
			return true
		}
		return byKey[i].info.ID < byKey[j].info.ID
	})

	out := make([]DetectorInfo, 0, len(byKey))
	for _, d := range byKey {
		out = append(out, d.info)
	}
	return out
}

func DetectorByID(id int) (DetectorInfo, bool) {
	if id <= 0 {
		return DetectorInfo{}, false
	}
	regMu.RLock()
	d, ok := registryByID[id]
	regMu.RUnlock()
	if !ok {
		return DetectorInfo{}, false
	}
	return d.info, true
}

func NewManager(getFrame GetFrameFunc, start StartFunc, stop StopFunc, isRecording IsRecordingFunc) *Manager {
	return &Manager{
		getFrame:    getFrame,
		start:       start,
		stop:        stop,
		isRecording: isRecording,
		workers:     make(map[string]*worker),
	}
}

func (m *Manager) Apply(rule Rule) {
	if rule.Interval <= 0 {
		rule.Interval = 250 * time.Millisecond
	}
	if rule.Post <= 0 {
		rule.Post = 10 * time.Second
	}

	m.Stop(rule.Src)

	if !rule.Enabled || rule.Src == "" {
		return
	}

	regMu.RLock()
	det, ok := registryByID[rule.TypeID]
	regMu.RUnlock()
	if !ok || det.factory == nil {
		return
	}

	w := &worker{
		rule:     rule,
		detector: det.factory(rule),
		stopCh:   make(chan struct{}),
	}

	m.mu.Lock()
	m.workers[rule.Src] = w
	m.mu.Unlock()

	go m.run(w)
}

func (m *Manager) Stop(src string) {
	if src == "" {
		return
	}
	m.mu.Lock()
	w, ok := m.workers[src]
	if ok {
		delete(m.workers, src)
	}
	m.mu.Unlock()
	if ok {
		close(w.stopCh)
	}
}

func (m *Manager) run(w *worker) {
	ticker := time.NewTicker(w.rule.Interval)
	defer ticker.Stop()

	var (
		prev       Frame
		hasPrev    bool
		prevAt     time.Time
		lastMotion time.Time
		active     bool
		moveCount  int
	)

	for {
		select {
		case <-w.stopCh:
			if active {
				m.stop(w.rule.Src)
			}
			return
		case <-ticker.C:
		}

		recording := m.isRecording != nil && m.isRecording(w.rule.Src)
		if active && !recording {
			active = false
			moveCount = 0
		}
		if !active && recording {
			active = true
		}

		raw, ok := m.getFrame(w.rule.Src)
		if !ok || raw.At.Equal(prevAt) {
			if active && !lastMotion.IsZero() && time.Since(lastMotion) >= w.rule.Post {
				m.stop(w.rule.Src)
				active = false
				moveCount = 0
			}
			continue
		}
		prevAt = raw.At

		frame, ok := normalizeFrame(raw)
		if !ok {
			continue
		}

		moved := false
		if hasPrev {
			moved = w.detector.Detect(prev, frame)
		}

		prev = frame
		hasPrev = true

		if moved {
			moveCount++
			lastMotion = time.Now()
			if !active && moveCount >= 1 {
				m.start(w.rule.Src)
				active = true
			}
			continue
		}
		moveCount = 0

		if active && !lastMotion.IsZero() && time.Since(lastMotion) >= w.rule.Post {
			m.stop(w.rule.Src)
			active = false
		}
	}
}

func normalizeFrame(raw RawFrame) (Frame, bool) {
	img, err := jpeg.Decode(bytes.NewReader(raw.Payload))
	if err != nil {
		if log.GetLevel() <= 0 {
			log.Debug().Err(err).Msg("trigger frame decode failed")
		}
		return Frame{}, false
	}

	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return Frame{}, false
	}

	const (
		w = 64
		h = 36
	)

	gray := make([]byte, w*h)
	for y := 0; y < h; y++ {
		sy := bounds.Min.Y + y*bounds.Dy()/h
		for x := 0; x < w; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/w
			r, g, b, _ := img.At(sx, sy).RGBA()
			gv := (299*r + 587*g + 114*b) / 1000 / 256
			gray[y*w+x] = byte(gv)
		}
	}

	return Frame{
		JPEG:       raw.Payload,
		JPEGWidth:  bounds.Dx(),
		JPEGHeight: bounds.Dy(),
		Gray:       gray,
		Width:      w,
		Height:     h,
		At:         raw.At,
	}, true
}
