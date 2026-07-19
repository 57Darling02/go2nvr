package record

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/AlexxIT/go2rtc/internal/streams"
	"github.com/AlexxIT/go2rtc/pkg/core"
)

type sessionTestProducer struct {
	core.Connection
	done chan struct{}
	once sync.Once
}

type blockingSessionProducer struct {
	core.Connection
	entered     chan struct{}
	release     chan struct{}
	done        chan struct{}
	enteredOnce sync.Once
	releaseOnce sync.Once
	doneOnce    sync.Once
}

func newBlockingSessionProducer() *blockingSessionProducer {
	codec := &core.Codec{Name: core.CodecH264, ClockRate: 90000, PayloadType: core.PayloadTypeRAW}
	return &blockingSessionProducer{
		Connection: core.Connection{Medias: []*core.Media{{
			Kind:      core.KindVideo,
			Direction: core.DirectionRecvonly,
			Codecs:    []*core.Codec{codec},
		}}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (p *blockingSessionProducer) GetMedias() []*core.Media {
	p.enteredOnce.Do(func() { close(p.entered) })
	<-p.release
	return p.Connection.GetMedias()
}

func (p *blockingSessionProducer) Start() error {
	<-p.done
	return nil
}

func (p *blockingSessionProducer) Stop() error {
	p.unblock()
	p.doneOnce.Do(func() { close(p.done) })
	return p.Connection.Stop()
}

func (p *blockingSessionProducer) unblock() {
	p.releaseOnce.Do(func() { close(p.release) })
}

func newSessionTestProducer() *sessionTestProducer {
	codec := &core.Codec{Name: core.CodecH264, ClockRate: 90000, PayloadType: core.PayloadTypeRAW}
	return &sessionTestProducer{
		Connection: core.Connection{Medias: []*core.Media{{
			Kind:      core.KindVideo,
			Direction: core.DirectionRecvonly,
			Codecs:    []*core.Codec{codec},
		}}},
		done: make(chan struct{}),
	}
}

func (p *sessionTestProducer) Start() error {
	<-p.done
	return nil
}

func (p *sessionTestProducer) Stop() error {
	p.once.Do(func() { close(p.done) })
	return p.Connection.Stop()
}

func TestSessionRebindsWhenStreamPointerChanges(t *testing.T) {
	useRecordTestDir(t)
	const name = "record-session-test"
	streams.HandleFunc("recordtest", func(string) (core.Producer, error) {
		return newSessionTestProducer(), nil
	})
	streams.Delete(name)
	t.Cleanup(func() { streams.Delete(name) })

	if _, err := streams.New(name, "recordtest:first"); err != nil {
		t.Fatal(err)
	}
	service := &recordService{sessions: make(map[string]*recordSession)}
	session := service.ensure(name)
	session.mu.Lock()
	session.manual = true
	session.mu.Unlock()
	session.reconcile()
	first := session.snapshot().Recorder
	if first == nil {
		t.Fatal("first recorder was not attached")
	}

	streams.Delete(name)
	if _, err := streams.New(name, "recordtest:replacement"); err != nil {
		t.Fatal(err)
	}
	session.reconcile()
	second := session.snapshot().Recorder
	if second == nil || second == first {
		t.Fatal("stream replacement did not create a new recorder")
	}
	if recording, _, _ := first.State(); recording {
		t.Fatal("old recorder remained active after stream replacement")
	}

	session.mu.Lock()
	session.manual = false
	session.mu.Unlock()
	session.reconcile()
	if session.snapshot().Recorder != nil {
		t.Fatal("recorder remained attached after its last intent was removed")
	}
}

func TestRecordStartReturnsAcceptedWhileAttaching(t *testing.T) {
	useRecordTestDir(t)
	const name = "record-session-slow-attach"
	producer := newBlockingSessionProducer()
	streams.HandleFunc("recordslow", func(string) (core.Producer, error) {
		return producer, nil
	})
	streams.Delete(name)
	if _, err := streams.New(name, "recordslow:camera"); err != nil {
		t.Fatal(err)
	}

	service := &recordService{sessions: make(map[string]*recordSession)}
	previous := recordSessions
	recordSessions = service
	t.Cleanup(func() {
		producer.unblock()
		if session := service.get(name); session != nil {
			session.mu.Lock()
			session.manual = false
			session.mu.Unlock()
			session.reconcile()
		}
		recordSessions = previous
		streams.Delete(name)
	})

	started := time.Now()
	request := httptest.NewRequest(http.MethodPost, "/api/record?src="+name+"&action=start", nil)
	response := httptest.NewRecorder()
	recordHandler(response, request)
	if elapsed := time.Since(started); elapsed > 200*time.Millisecond {
		t.Fatalf("start blocked for slow attach: %v", elapsed)
	}
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusAccepted)
	}
	if state := service.snapshot(name); state.Phase != "attaching" || state.Recorder != nil {
		t.Fatalf("unexpected pending state: %#v", state)
	}

	select {
	case <-producer.entered:
	case <-time.After(time.Second):
		t.Fatal("background attach did not start")
	}
}
