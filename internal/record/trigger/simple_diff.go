package trigger

import "time"

// static params for detector
// detector will try to find it in config file
var simpleDiffRuleParams = []DetectorParam{
	{
		Key:          "threshold",
		Type:         "number",
		DefaultValue: 14,
		Min:          intPtr(1),
		Max:          intPtr(255),
		Tip:          "Average grayscale diff threshold treated as motion.",
	},
	{
		Key:          "post_sec",
		Type:         "number",
		DefaultValue: 10,
		Min:          intPtr(1),
		Tip:          "Keep recording for N seconds after last detected motion.",
	},
	{
		Key:          "min_hits",
		Type:         "number",
		DefaultValue: 1,
		Min:          intPtr(1),
		Tip:          "Consecutive motion hits required before entering active state.",
	},
}


// define var type which will be used directly by detector
type SimpleDiffDetector struct {
	threshold int
	post      time.Duration
	minHits   int

	hits        int
	lastHit     time.Time
	lastFrameAt time.Time
	active      bool
}


// Gets static params from a configuration file and assigns it to detector instance
func NewSimpleDiffDetector(rule Rule) Detector {
	parsed := rule.ParseBySchema(simpleDiffRuleParams)

	return &SimpleDiffDetector{
		threshold: parsed["threshold"].(int),
		post:      time.Duration(parsed["post_sec"].(int)) * time.Second,
		minHits:   parsed["min_hits"].(int),
	}
}

// Detect() returns recorder target state, not just motion/no-motion. Result means recording or not
func (d *SimpleDiffDetector) Detect(prev, cur *Frame, isRecording bool) bool {
	now := time.Now()

	// No new frame this tick.
	// Default policy: keep current recorder state.
	// if stream stays silent for too long, force stop.
	if cur == nil {
		if isRecording && !d.lastFrameAt.IsZero() && now.Sub(d.lastFrameAt) >= d.post {
			d.active = false
			d.hits = 0
			return false
		}
		return isRecording
	}

	d.lastFrameAt = cur.At
	if prev == nil {
		// Warm-up stage: no diff can be computed yet.
		return isRecording
	}

	score := simpleDiffScore(prev.Gray, cur.Gray)
	if log.GetLevel() <= 0 {
		log.Debug().Msgf("simple_diff score=%d threshold=%d", score, d.threshold)
	}

	if score >= d.threshold {
		d.hits++
		d.lastHit = now
		if d.hits >= d.minHits {
			d.active = true
		}
		return d.active
	}

	d.hits = 0
	if d.active && !d.lastHit.IsZero() && now.Sub(d.lastHit) >= d.post {
		d.active = false
	}
	return d.active
}

func simpleDiffScore(a, b []byte) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	n := len(a)
	if len(b) < n {
		n = len(b)
	}

	sum := 0
	for i := 0; i < n; i++ {
		diff := int(a[i]) - int(b[i])
		if diff < 0 {
			diff = -diff
		}
		sum += diff
	}
	return sum / n
}

func init() {
	Register(1, "simple_diff", "Simple Diff", simpleDiffRuleParams, NewSimpleDiffDetector)
}

func intPtr(v int) *int {
	return &v
}
