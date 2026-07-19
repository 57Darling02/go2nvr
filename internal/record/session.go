package record

import (
	"errors"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/internal/streams"
)

var errRecordStreamNotFound = errors.New("record stream not found")

var retrySchedule = [...]time.Duration{
	time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
	60 * time.Second,
}

type recordService struct {
	mu       sync.RWMutex
	sessions map[string]*recordSession
}

type recordSession struct {
	src string

	// opMu serializes attach/detach and all calls that may synchronously dial a
	// stream. State remains readable while an attach is in progress.
	opMu sync.Mutex

	// reconcileMu coalesces background reconciliation requests. A failed dial or
	// slow disk detach must not turn the one-second reconciliation ticker into an
	// unbounded queue of goroutines waiting on opMu.
	reconcileMu      sync.Mutex
	reconciling      bool
	reconcilePending bool

	mu sync.RWMutex

	stream    *streams.Stream
	recorder  *Recorder
	manual    bool
	triggered bool
	hasRule   bool
	prebuffer time.Duration
	trigger   bool

	phase      string
	lastError  string
	retryAt    time.Time
	stopReason string
	attempts   int
}

type sessionSnapshot struct {
	Recorder         *Recorder
	Phase            string
	DesiredRecording bool
	LastError        string
	RetryAt          time.Time
	StopReason       string
	Attached         bool
}

var recordSessions = &recordService{sessions: make(map[string]*recordSession)}

func (s *recordService) get(src string) *recordSession {
	s.mu.RLock()
	session := s.sessions[src]
	s.mu.RUnlock()
	return session
}

func (s *recordService) ensure(src string) *recordSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.sessions[src]; session != nil {
		return session
	}
	session := &recordSession{src: src, phase: "stopped"}
	s.sessions[src] = session
	return session
}

func (s *recordService) list() map[string]*recordSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*recordSession, len(s.sessions))
	for src, session := range s.sessions {
		out[src] = session
	}
	return out
}

func (s *recordService) reconcileLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for _, session := range s.list() {
			session.scheduleReconcile()
		}
	}
}

func (s *recordService) applyRule(rule recordRule) {
	session := s.ensure(rule.Src)
	session.mu.Lock()
	session.hasRule = true
	session.prebuffer = rule.prebufferDuration()
	session.trigger = rule.triggerEnabled()
	recorder := session.recorder
	session.mu.Unlock()
	if recorder != nil {
		recorder.SetPrebuffer(rule.prebufferDuration())
		recorder.SetSnapshotRequired(rule.triggerEnabled())
	}
	session.scheduleReconcile()
}

func (s *recordService) removeRule(src string) {
	session := s.get(src)
	if session == nil {
		return
	}
	session.mu.Lock()
	session.hasRule = false
	session.trigger = false
	session.triggered = false
	session.prebuffer = 0
	recorder := session.recorder
	session.mu.Unlock()
	if recorder != nil {
		recorder.SetPrebuffer(0)
		recorder.SetSnapshotRequired(false)
	}
	session.scheduleReconcile()
}

func (s *recordService) startManual(src string, prebuffer time.Duration) (sessionSnapshot, error) {
	if streams.Get(src) == nil {
		return sessionSnapshot{}, errRecordStreamNotFound
	}
	session := s.ensure(src)
	session.mu.Lock()
	session.manual = true
	if !session.hasRule {
		session.prebuffer = prebuffer
	}
	session.stopReason = ""
	recorder := session.recorder
	phase := session.phase
	retryAt := session.retryAt
	if recorder == nil && !retryAt.After(time.Now()) {
		session.phase = "attaching"
		phase = "attaching"
	}
	session.mu.Unlock()

	// Dialing a producer may take seconds. An initial attach, retry, or a
	// request that races a drain is deliberately asynchronous so the API can
	// report its transitional state instead of holding an HTTP worker.
	if recorder == nil || phase == "attaching" || phase == "draining" || retryAt.After(time.Now()) {
		session.scheduleReconcile()
		return session.snapshot(), nil
	}

	session.reconcile()
	state := session.snapshot()
	if state.Phase == "backoff" || state.Phase == "attaching" {
		return state, nil
	}
	if state.Recorder == nil {
		return state, errors.New("recorder is unavailable")
	}
	return state, nil
}

func (s *recordService) stopManual(src string) (sessionSnapshot, error) {
	if streams.Get(src) == nil {
		return sessionSnapshot{}, errRecordStreamNotFound
	}
	session := s.get(src)
	if session == nil {
		return sessionSnapshot{Phase: "stopped"}, nil
	}
	session.mu.Lock()
	session.manual = false
	session.stopReason = "manual"
	recorder := session.recorder
	phase := session.phase
	hasRule := session.hasRule
	if recorder == nil && phase == "draining" {
		session.phase = "draining"
		phase = "draining"
	} else if recorder == nil && phase == "attaching" && !hasRule {
		session.phase = "draining"
		phase = "draining"
	}
	session.mu.Unlock()

	if recorder == nil {
		if phase == "draining" || phase == "attaching" {
			session.scheduleReconcile()
			return session.snapshot(), nil
		}
		session.reconcile()
		return session.snapshot(), nil
	}

	recording, _, _ := recorder.State()
	if recording || !hasRule || phase == "attaching" || phase == "draining" {
		session.mu.Lock()
		if session.recorder == recorder {
			session.phase = "draining"
		}
		session.mu.Unlock()
		session.scheduleReconcile()
		return session.snapshot(), nil
	}

	session.reconcile()
	return session.snapshot(), nil
}

func (s *recordService) startTrigger(src string) {
	session := s.ensure(src)
	session.mu.Lock()
	session.triggered = true
	session.stopReason = ""
	session.mu.Unlock()
	session.scheduleReconcile()
}

func (s *recordService) stopTrigger(src string) {
	session := s.get(src)
	if session == nil {
		return
	}
	session.mu.Lock()
	session.triggered = false
	session.stopReason = "trigger"
	session.mu.Unlock()
	session.scheduleReconcile()
}

func (s *recordService) recorder(src string) *Recorder {
	session := s.get(src)
	if session == nil {
		return nil
	}
	return session.snapshot().Recorder
}

func (s *recordService) ensureTriggerAttachment(src string) *Recorder {
	session := s.get(src)
	if session == nil {
		return nil
	}
	session.scheduleReconcile()
	return session.snapshot().Recorder
}

func (s *recordService) isRecording(src string) bool {
	recorder := s.recorder(src)
	if recorder == nil {
		return false
	}
	recording, _, _ := recorder.State()
	return recording
}

func (s *recordService) snapshot(src string) sessionSnapshot {
	if session := s.get(src); session != nil {
		return session.snapshot()
	}
	return sessionSnapshot{Phase: "stopped"}
}

func (s *recordService) reconcileRule(src string, rule recordRule) {
	s.applyRule(rule)
	if rule.triggerEnabled() {
		return
	}
	// A rule can be edited while its trigger is active. Clearing the desired
	// trigger state here prevents a removed detector from keeping a segment open.
	s.stopTrigger(src)
}

func (s *recordSession) snapshot() sessionSnapshot {
	s.mu.RLock()
	state := sessionSnapshot{
		Recorder:         s.recorder,
		Phase:            s.phase,
		DesiredRecording: s.manual || s.triggered,
		LastError:        s.lastError,
		RetryAt:          s.retryAt,
		StopReason:       s.stopReason,
		Attached:         s.recorder != nil,
	}
	s.mu.RUnlock()
	return state
}

func (s *recordSession) scheduleReconcile() {
	s.reconcileMu.Lock()
	s.reconcilePending = true
	if s.reconciling {
		s.reconcileMu.Unlock()
		return
	}
	s.reconciling = true
	s.reconcileMu.Unlock()

	go func() {
		for {
			s.reconcileMu.Lock()
			s.reconcilePending = false
			s.reconcileMu.Unlock()

			s.reconcile()

			s.reconcileMu.Lock()
			if !s.reconcilePending {
				s.reconciling = false
				s.reconcileMu.Unlock()
				return
			}
			s.reconcileMu.Unlock()
		}
	}()
}

func (s *recordSession) reconcile() {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	now := time.Now()
	s.mu.RLock()
	shouldAttach := s.manual || s.triggered || s.hasRule
	desiredRecording := s.manual || s.triggered
	boundStream := s.stream
	recorder := s.recorder
	retryAt := s.retryAt
	prebuffer := s.prebuffer
	snapshotRequired := s.trigger
	s.mu.RUnlock()

	if !shouldAttach {
		if recorder != nil {
			s.detach(boundStream, recorder)
		}
		s.mu.Lock()
		if s.recorder == recorder {
			s.recorder = nil
			s.stream = nil
			s.phase = "stopped"
		}
		s.mu.Unlock()
		return
	}

	current := streams.Get(s.src)
	if current == nil {
		if recorder != nil {
			s.detach(boundStream, recorder)
		}
		s.mu.Lock()
		if s.recorder == recorder {
			s.recorder = nil
			s.stream = nil
			s.phase = "unavailable"
			s.lastError = errRecordStreamNotFound.Error()
		}
		s.mu.Unlock()
		return
	}

	if recorder != nil && boundStream != current {
		s.detach(boundStream, recorder)
		s.mu.Lock()
		if s.recorder == recorder {
			s.recorder = nil
			s.stream = nil
		}
		s.mu.Unlock()
		recorder = nil
	}

	if retryAt.After(now) {
		s.mu.Lock()
		s.phase = "backoff"
		s.mu.Unlock()
		return
	}

	if recorder == nil {
		s.mu.Lock()
		s.phase = "attaching"
		s.mu.Unlock()

		recorder = newRecorder(s.src, prebuffer)
		recorder.SetSnapshotRequired(snapshotRequired)
		recorder.SetFailureHandler(s.recorderFailed)
		if err := current.AddConsumer(recorder); err != nil {
			_ = recorder.Stop()
			s.fail(err, "attach")
			return
		}
		s.mu.Lock()
		s.recorder = recorder
		s.stream = current
		s.phase = "idle"
		s.lastError = ""
		s.retryAt = time.Time{}
		s.attempts = 0
		s.mu.Unlock()
	}

	// Intent may have changed while AddConsumer was dialing. Observe it again
	// before opening or closing a segment.
	s.mu.RLock()
	desiredRecording = s.manual || s.triggered
	shouldAttach = s.manual || s.triggered || s.hasRule
	s.mu.RUnlock()
	if !shouldAttach {
		s.detach(current, recorder)
		s.mu.Lock()
		if s.recorder == recorder {
			s.recorder, s.stream, s.phase = nil, nil, "stopped"
		}
		s.mu.Unlock()
		return
	}
	recording, _, _ := recorder.State()
	if desiredRecording {
		if recording {
			s.mu.Lock()
			if s.recorder == recorder {
				s.phase = "recording"
			}
			s.mu.Unlock()
			return
		}
		if err := recorder.StartRecording(); err != nil {
			s.failRecorder(recorder, err, "start")
			return
		}
		s.mu.Lock()
		if s.recorder == recorder {
			s.phase = "recording"
		}
		s.mu.Unlock()
		return
	}
	if !recording {
		s.mu.Lock()
		if s.recorder == recorder {
			s.phase = "idle"
		}
		s.mu.Unlock()
		return
	}
	if err := recorder.StopRecording(); err != nil {
		s.failRecorder(recorder, err, "stop")
		return
	}
	s.mu.Lock()
	if s.recorder == recorder {
		s.phase = "idle"
	}
	s.mu.Unlock()
}

func (s *recordSession) detach(stream *streams.Stream, recorder *Recorder) {
	if stream != nil {
		stream.RemoveConsumer(recorder)
		return
	}
	_ = recorder.Stop()
}

func (s *recordSession) fail(err error, reason string) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.attempts++
	delay := retrySchedule[len(retrySchedule)-1]
	if s.attempts <= len(retrySchedule) {
		delay = retrySchedule[s.attempts-1]
	}
	s.phase = "backoff"
	s.lastError = err.Error()
	s.stopReason = reason
	s.retryAt = time.Now().Add(delay)
	s.mu.Unlock()
}

func (s *recordSession) recorderFailed(recorder *Recorder, err error) {
	reason := "write"
	if errors.Is(err, errRecorderBackpressure) {
		reason = "backpressure"
	}
	s.failRecorder(recorder, err, reason)
}

func (s *recordSession) failRecorder(recorder *Recorder, err error, reason string) {
	s.mu.RLock()
	current := s.recorder
	s.mu.RUnlock()
	if current != recorder {
		return
	}
	s.fail(err, reason)

	// Writer failures invoke this path from the recorder loop. Detach in a
	// separate goroutine so RemoveConsumer can wait for Recorder.Stop without
	// waiting on that same loop.
	go func() {
		s.opMu.Lock()
		defer s.opMu.Unlock()
		s.mu.RLock()
		if s.recorder != recorder {
			s.mu.RUnlock()
			return
		}
		stream := s.stream
		s.mu.RUnlock()
		s.detach(stream, recorder)
		s.mu.Lock()
		if s.recorder == recorder {
			s.recorder = nil
			s.stream = nil
		}
		s.mu.Unlock()
	}()
}
