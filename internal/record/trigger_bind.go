package record

import rectrigger "github.com/AlexxIT/go2rtc/internal/record/trigger"

var triggerManager *rectrigger.Manager

func initTrigger() {
	rectrigger.SetLogger(log)
	triggerManager = rectrigger.NewManager(getTriggerFrame, Start, Stop, isRecording)
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
	triggerManager.Stop(src)
}

func getTriggerFrame(src string) (rectrigger.RawFrame, bool) {
	mu.RLock()
	rec := recorders[src]
	mu.RUnlock()
	if rec == nil {
		return rectrigger.RawFrame{}, false
	}
	b, at, ok := rec.LastKeyframe()
	if !ok {
		return rectrigger.RawFrame{}, false
	}
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
