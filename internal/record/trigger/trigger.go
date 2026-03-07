package trigger

import (
	"bytes"
	"image/jpeg"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Logger
var log zerolog.Logger

func SetLogger(l zerolog.Logger) {
	log = l
}

// Core Types
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
	Prebuffer int
	Interval  time.Duration
	Params    map[string]interface{}
}

type DetectorParam struct {
	Key          string      `json:"key"`
	Type         string      `json:"type"`
	DefaultValue interface{} `json:"default,omitempty"`
	Min          *int        `json:"min,omitempty"`
	Max          *int        `json:"max,omitempty"`
	Tip          string      `json:"tip,omitempty"`
}

type DetectorInfo struct {
	ID     int             `json:"id"`
	Key    string          `json:"key"`
	Name   string          `json:"name"`
	Params []DetectorParam `json:"params,omitempty"`
}

// Contracts
type Detector interface {
	Detect(prev, cur *Frame, isRecording bool) bool
}

type DetectorFactory func(rule Rule) Detector

type GetFrameFunc func(src string) (RawFrame, bool)
type StartFunc func(src string)
type StopFunc func(src string)
type IsRecordingFunc func(src string) bool

// Rule Param Parsing

// NumberParam parses one numeric param from rule params.
// If parsing fails or value is out of [min,max], it returns schema default.
func (r Rule) NumberParam(spec DetectorParam) int {
	def, ok := numberFromValue(spec.DefaultValue)
	if !ok {
		return 0
	}
	if len(r.Params) == 0 {
		return def
	}
	raw, has := r.Params[spec.Key]
	if !has {
		return def
	}
	v, ok := numberFromValue(raw)
	if !ok {
		return def
	}
	if spec.Min != nil && v < *spec.Min {
		return def
	}
	if spec.Max != nil && v > *spec.Max {
		return def
	}
	return v
}

// StringParam parses one string param from rule params.
// Empty string returns schema default.
func (r Rule) StringParam(spec DetectorParam) string {
	def, _ := spec.DefaultValue.(string)
	if len(r.Params) == 0 {
		return def
	}
	raw, has := r.Params[spec.Key]
	if !has {
		return def
	}
	s, ok := raw.(string)
	if !ok {
		return def
	}
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// ParseBySchema parses all params declared in schema by each param type.
// Supported types:
// - "number" => int
// - "string" => string
func (r Rule) ParseBySchema(schema []DetectorParam) map[string]interface{} {
	out := make(map[string]interface{}, len(schema))
	for _, spec := range schema {
		switch strings.ToLower(spec.Type) {
		case "number":
			out[spec.Key] = r.NumberParam(spec)
		case "string":
			out[spec.Key] = r.StringParam(spec)
		}
	}
	return out
}

func numberFromValue(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		if strings.TrimSpace(n) == "" {
			return 0, false
		}
		i, err := strconv.Atoi(n)
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

// Detector Registry
type registeredDetector struct {
	info    DetectorInfo
	factory DetectorFactory
}

var (
	regMu         sync.RWMutex
	registryByKey = map[string]registeredDetector{}
	registryByID  = map[int]registeredDetector{}
)

func Register(id int, key, name string, params []DetectorParam, factory DetectorFactory) {
	if key == "" || factory == nil {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()

	if _, exists := registryByKey[key]; exists {
		log.Warn().Str("key", key).Msg("trigger register ignored: duplicate key")
		return
	}
	if id > 0 {
		if _, exists := registryByID[id]; exists {
			log.Warn().Int("id", id).Str("key", key).Msg("trigger register ignored: duplicate id")
			return
		}
	}

	d := registeredDetector{
		info: DetectorInfo{
			ID:     id,
			Key:    key,
			Name:   name,
			Params: copyDetectorParams(params),
		},
		factory: factory,
	}
	registryByKey[key] = d
	if id > 0 {
		registryByID[id] = d
	}
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

func copyDetectorParams(in []DetectorParam) []DetectorParam {
	if len(in) == 0 {
		return nil
	}
	out := make([]DetectorParam, len(in))
	copy(out, in)
	return out
}

// Manager
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
		rule.Interval = 400 * time.Millisecond
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
		prev    Frame
		hasPrev bool
		prevAt  time.Time
	)

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
		}

		isRecording := false
		if m.isRecording != nil {
			isRecording = m.isRecording(w.rule.Src)
		}

		var (
			prevPtr *Frame
			curPtr  *Frame
		)
		if hasPrev {
			prevCopy := prev
			prevPtr = &prevCopy
		}

		raw, ok := m.getFrame(w.rule.Src)
		if ok && !raw.At.Equal(prevAt) {
			prevAt = raw.At

			frame, normOK := normalizeFrame(raw)
			if normOK {
				frameCopy := frame
				curPtr = &frameCopy
			}
		}

		target := w.detector.Detect(prevPtr, curPtr, isRecording)
		if target != isRecording {
			if target {
				m.start(w.rule.Src)
			} else {
				m.stop(w.rule.Src)
			}
		}

		if curPtr != nil {
			prev = *curPtr
			hasPrev = true
		}
	}
}

// Frame Normalize
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
