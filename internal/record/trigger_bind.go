package record

import (
	"sync"
	"time"

	rectrigger "github.com/AlexxIT/go2rtc/internal/record/trigger"
)

var triggerManager *rectrigger.Manager
var triggerAttachState struct {
	mu   sync.Mutex
	next map[string]time.Time
}

const triggerAttachRetryInterval = 2 * time.Second

func initTrigger() {
	rectrigger.SetLogger(log)
	triggerManager = rectrigger.NewManager(getTriggerFrame, Start, Stop, isRecording)
	triggerAttachState.next = make(map[string]time.Time)
}

func startTriggerForRule(rule recordRule) {
	if triggerManager == nil {
		return
	}
	triggerManager.Apply(rectrigger.Rule{
		Src:       rule.Src,
		Enabled:   rule.triggerEnabled(),
		TypeID:    rule.triggerID(),
		Threshold: rule.triggerThreshold(),
		Post:      rule.triggerPostDuration(),
		Interval:  rule.triggerInterval(),
	})
}

func stopTrigger(src string) {
	if triggerManager == nil {
		return
	}
	resetTriggerAttachState(src)
	triggerManager.Stop(src)
}

func getTriggerFrame(src string) (rectrigger.RawFrame, bool) {
	mu.RLock()
	rec := recorders[src]
	mu.RUnlock()
	if rec == nil {
		rec = ensureRecorderForTrigger(src)
		if rec == nil {
			return rectrigger.RawFrame{}, false
		}
	}
	b, at, ok := rec.LastKeyframe()
	if !ok {
		// Recorder may be attached but still warming up after reconnect.
		_ = ensureRecorderForTrigger(src)
		return rectrigger.RawFrame{}, false
	}
	resetTriggerAttachState(src)
	return rectrigger.RawFrame{Payload: b, At: at}, true
}

func isRecording(src string) bool {
	mu.RLock()
	rec := recorders[src]
	mu.RUnlock()
	if rec == nil {
		return false
	}
	recording, _, _ := rec.State()
	return recording
}

func ensureRecorderForTrigger(src string) *Recorder {
	rule, ok := getRule(src)
	if !ok {
		return nil
	}

	now := time.Now()
	// Throttle attach attempts to avoid hot-looping when source is offline.
	triggerAttachState.mu.Lock()
	nextAt, has := triggerAttachState.next[src]
	if has && now.Before(nextAt) {
		triggerAttachState.mu.Unlock()
		return nil
	}
	triggerAttachState.next[src] = now.Add(triggerAttachRetryInterval)
	triggerAttachState.mu.Unlock()

	rec := ensureRecorder(src, rule.prebufferDuration())
	if rec != nil {
		resetTriggerAttachState(src)
	}
	return rec
}

func resetTriggerAttachState(src string) {
	triggerAttachState.mu.Lock()
	delete(triggerAttachState.next, src)
	triggerAttachState.mu.Unlock()
}
