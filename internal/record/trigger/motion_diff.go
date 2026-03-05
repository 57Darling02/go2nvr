package trigger

type MotionDiffDetector struct {
	threshold int
}

func NewMotionDiffDetector(rule Rule) Detector {
	threshold := rule.Threshold
	if threshold <= 0 {
		threshold = 14
	}
	return &MotionDiffDetector{threshold: threshold}
}

func (d *MotionDiffDetector) Detect(prev, cur Frame) bool {
	score := grayDiffScore(prev.Gray, cur.Gray)
	if log.GetLevel() <= 0 {
		log.Debug().Int("score", score).Int("threshold", d.threshold).Msg("motion_diff detection")
	}
	return score >= d.threshold
}

func grayDiffScore(a, b []byte) int {
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
	Register(1, "motion_diff", "Motion Diff", NewMotionDiffDetector)
}
