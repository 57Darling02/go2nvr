package record

import (
	"sync"
)

const megabyte = int64(1024 * 1024)

type recordLimits struct {
	MemoryMB        int `yaml:"memory_mb"`
	PrebufferMB     int `yaml:"prebuffer_mb"`
	WriterQueueMB   int `yaml:"writer_queue_mb"`
	SnapshotWorkers int `yaml:"snapshot_workers"`
}

var defaultRecordLimits = recordLimits{
	MemoryMB:        256,
	PrebufferMB:     32,
	WriterQueueMB:   16,
	SnapshotWorkers: 1,
}

func normalizeRecordLimits(in recordLimits) (recordLimits, bool) {
	defaults := defaultRecordLimits
	if in.MemoryMB == 0 {
		in.MemoryMB = defaults.MemoryMB
	}
	if in.PrebufferMB == 0 {
		in.PrebufferMB = defaults.PrebufferMB
	}
	if in.WriterQueueMB == 0 {
		in.WriterQueueMB = defaults.WriterQueueMB
	}
	if in.SnapshotWorkers == 0 {
		in.SnapshotWorkers = defaults.SnapshotWorkers
	}
	if in.MemoryMB < 1 || in.PrebufferMB < 1 || in.WriterQueueMB < 1 || in.SnapshotWorkers < 1 || in.SnapshotWorkers > 4 ||
		in.MemoryMB < in.PrebufferMB+in.WriterQueueMB+2 {
		return defaults, false
	}
	return in, true
}

func (l recordLimits) memoryBytes() int64 {
	return int64(l.MemoryMB) * megabyte
}

func (l recordLimits) prebufferBytes() int64 {
	return int64(l.PrebufferMB) * megabyte
}

func (l recordLimits) writerQueueBytes() int64 {
	return int64(l.WriterQueueMB) * megabyte
}

func currentLimits() recordLimits {
	confMu.RLock()
	defer confMu.RUnlock()
	limits, valid := normalizeRecordLimits(cfg.Mod.Limits)
	if !valid {
		return defaultRecordLimits
	}
	return limits
}

type memoryBudget struct {
	mu    sync.Mutex
	limit int64
	used  int64
}

func newMemoryBudget(limit int64) *memoryBudget {
	return &memoryBudget{limit: limit}
}

func (b *memoryBudget) reserve(size int64) bool {
	if size <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if size > b.limit-b.used {
		return false
	}
	b.used += size
	return true
}

func (b *memoryBudget) release(size int64) {
	if size <= 0 {
		return
	}
	b.mu.Lock()
	b.used -= size
	if b.used < 0 {
		b.used = 0
	}
	b.mu.Unlock()
}

func (b *memoryBudget) setLimit(limit int64) {
	b.mu.Lock()
	b.limit = limit
	b.mu.Unlock()
}

func (b *memoryBudget) usage() (used, limit int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used, b.limit
}

var recordMemory = newMemoryBudget(defaultRecordLimits.memoryBytes())
