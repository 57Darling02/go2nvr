package record

import rectrigger "github.com/AlexxIT/go2rtc/internal/record/trigger"

var triggerManager *rectrigger.Manager

func initTrigger() {
	rectrigger.SetLogger(log)
	triggerManager = rectrigger.NewManager(getTriggerFrame, recordSessions.startTrigger, recordSessions.stopTrigger, recordSessions.isRecording)
}

func startTriggerForRule(rule recordRule) {
	if triggerManager == nil {
		return
	}
	triggerManager.Apply(rectrigger.Rule{
		Src:       rule.Src,
		Enabled:   rule.triggerEnabled(),
		TypeID:    rule.triggerID(),
		Prebuffer: rule.Prebuffer,
		Interval:  rule.triggerInterval(),
		Params:    rule.TriggerParams,
	})
}

func stopTrigger(src string) {
	if triggerManager != nil {
		triggerManager.Stop(src)
	}
}

func getTriggerFrame(src string) (rectrigger.RawFrame, bool) {
	recorder := recordSessions.recorder(src)
	if recorder == nil {
		recorder = recordSessions.ensureTriggerAttachment(src)
		if recorder == nil {
			return rectrigger.RawFrame{}, false
		}
	}
	payload, at, ok := recorder.LastKeyframe()
	if !ok {
		return rectrigger.RawFrame{}, false
	}
	return rectrigger.RawFrame{Payload: payload, At: at}, true
}
