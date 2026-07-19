package record

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pion/rtp"
)

func TestRecordMailboxDrainsBeforeBackpressure(t *testing.T) {
	mailbox := newRecordMailbox(10)
	first := recordEvent{kind: recordPacket, packet: &packetData{size: 6}}
	if !mailbox.pushPacket(first) {
		t.Fatal("first packet was rejected")
	}
	if mailbox.pushPacket(recordEvent{kind: recordPacket, packet: &packetData{size: 5}}) {
		t.Fatal("overflow packet was accepted")
	}
	if !mailbox.pushControl(recordEvent{kind: recordStop}) {
		t.Fatal("control event was rejected")
	}
	mailbox.signalOverflow()

	for index, want := range []recordEventKind{recordPacket, recordStop, recordOverflow} {
		event, ok := mailbox.next()
		if !ok || event.kind != want {
			t.Fatalf("event %d = (%v, %v), want %v", index, event.kind, ok, want)
		}
	}
}

func TestRecordMailboxCloseGatesNewIngress(t *testing.T) {
	mailbox := newRecordMailbox(1024)
	if !mailbox.pushPacket(recordEvent{kind: recordPacket, packet: &packetData{size: 1}}) {
		t.Fatal("initial packet was rejected")
	}
	if !mailbox.pushControl(recordEvent{kind: recordClose}) {
		t.Fatal("close marker was rejected")
	}
	if mailbox.pushPacket(recordEvent{kind: recordPacket, packet: &packetData{size: 1}}) {
		t.Fatal("packet was accepted after close marker")
	}
	if mailbox.pushControl(recordEvent{kind: recordStart}) {
		t.Fatal("control event was accepted after close marker")
	}
}

func TestRecorderReleasesPacketRejectedByCloseMarker(t *testing.T) {
	before, _ := recordMemory.usage()
	recorder := &Recorder{mailbox: newRecordMailbox(1024)}
	recorder.stateMu.Lock()
	recorder.recording = true
	recorder.stateMu.Unlock()
	recorder.accepting.Store(true)
	if !recorder.mailbox.pushControl(recordEvent{kind: recordClose}) {
		t.Fatal("close marker was rejected")
	}

	recorder.writeRTP(0, true, true, &rtp.Packet{Payload: []byte{1, 2, 3}})
	after, _ := recordMemory.usage()
	if after != before {
		t.Fatalf("packet rejected after close leaked memory: before=%d after=%d", before, after)
	}
}

func TestLatestSnapshotKeepsPendingThumbnailTarget(t *testing.T) {
	pool := newSnapshotPool()
	recorder := &Recorder{}
	before, _ := recordMemory.usage()
	if !recordMemory.reserve(10) {
		t.Fatal("could not reserve first snapshot")
	}
	pool.submit(recorder, snapshotTask{bytes: 10, thumbnail: "/records/clip.mp4"})
	if !recordMemory.reserve(20) {
		t.Fatal("could not reserve replacement snapshot")
	}
	pool.submit(recorder, snapshotTask{bytes: 20})

	pool.mu.Lock()
	pending := pool.pending[recorder]
	pool.mu.Unlock()
	if pending.thumbnail != "/records/clip.mp4" {
		t.Fatalf("thumbnail target = %q, want original target", pending.thumbnail)
	}
	pool.cancel(recorder)
	after, _ := recordMemory.usage()
	if after != before {
		t.Fatalf("snapshot replacement leaked memory: before=%d after=%d", before, after)
	}
}

func TestPrebufferDropsUndecodableHistory(t *testing.T) {
	useRecordTestDir(t)
	before, _ := recordMemory.usage()
	recorder := &Recorder{}
	recorder.SetPrebuffer(time.Second)
	now := time.Now()

	// Audio and non-key video cannot form a decodable recording start.
	recorder.appendPrebuffer(&packetData{at: now, size: 10})
	recorder.appendPrebuffer(&packetData{at: now, video: true, size: 10})
	if len(recorder.prebufferPackets) != 0 {
		t.Fatal("undecodable packets were retained")
	}

	// Once the only keyframe falls outside the configured time window, the
	// buffer is discarded instead of growing forever waiting for another IDR.
	recorder.appendPrebuffer(&packetData{at: now, video: true, keyframe: true, size: 10})
	recorder.appendPrebuffer(&packetData{at: now.Add(2 * time.Second), video: true, size: 10})
	if len(recorder.prebufferPackets) != 0 || recorder.prebufferBytes != 0 {
		t.Fatalf("stale undecodable buffer retained: %#v", recorder.prebufferPackets)
	}
	after, _ := recordMemory.usage()
	if after != before {
		t.Fatalf("memory budget leaked: before=%d after=%d", before, after)
	}
}

func TestNormalizeRecordLimits(t *testing.T) {
	valid, ok := normalizeRecordLimits(recordLimits{
		MemoryMB:        64,
		PrebufferMB:     32,
		WriterQueueMB:   16,
		SnapshotWorkers: 2,
	})
	if !ok || valid.MemoryMB != 64 || valid.SnapshotWorkers != 2 {
		t.Fatalf("valid limits normalized unexpectedly: %#v, %v", valid, ok)
	}
	invalid, ok := normalizeRecordLimits(recordLimits{MemoryMB: 32, PrebufferMB: 32, WriterQueueMB: 16})
	if ok || invalid != defaultRecordLimits {
		t.Fatalf("invalid limits = %#v, %v", invalid, ok)
	}
}

func TestRecordActionRejectsUnknownStream(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/record?src=missing-stream&action=start", nil)
	response := httptest.NewRecorder()
	recordHandler(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRecordStateRejectsUnknownStream(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/record?src=missing-stream", nil)
	response := httptest.NewRecorder()
	recordHandler(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestUnavailableRuleReportsStoppedStatus(t *testing.T) {
	useRecordTestDir(t)
	const name = "offline-camera"
	previousSessions := recordSessions
	service := &recordService{sessions: make(map[string]*recordSession)}
	recordSessions = service
	t.Cleanup(func() { recordSessions = previousSessions })

	confMu.Lock()
	cfg.Mod.Rules = []recordRule{{Src: name}}
	confMu.Unlock()
	session := service.ensure(name)
	session.mu.Lock()
	session.hasRule = true
	session.phase = "unavailable"
	session.lastError = errRecordStreamNotFound.Error()
	session.mu.Unlock()

	state := buildState(name)
	if state["status"] != "stopped" || state["phase"] != "unavailable" {
		t.Fatalf("unavailable rule state = %#v", state)
	}
}
