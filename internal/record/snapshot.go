package record

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/internal/ffmpeg"
	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/h264/annexb"
	"github.com/AlexxIT/go2rtc/pkg/mjpeg"
)

const snapshotDeadline = 5 * time.Second

type snapshotTask struct {
	codec     string
	payload   []byte
	at        time.Time
	bytes     int64
	thumbnail string
}

// snapshotPool keeps at most one queued frame for a recorder. The workers are
// shared because a burst of offline cameras must not start a burst of ffmpeg
// processes.
type snapshotPool struct {
	mu      sync.Mutex
	cond    *sync.Cond
	pending map[*Recorder]snapshotTask
	queued  map[*Recorder]bool
	queue   []*Recorder
	workers int
}

func newSnapshotPool() *snapshotPool {
	p := &snapshotPool{
		pending: make(map[*Recorder]snapshotTask),
		queued:  make(map[*Recorder]bool),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

var snapshots = newSnapshotPool()

func (p *snapshotPool) configure(workers int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for p.workers < workers {
		p.workers++
		go p.run()
	}
}

func (p *snapshotPool) submit(rec *Recorder, task snapshotTask) {
	p.mu.Lock()
	if previous, ok := p.pending[rec]; ok {
		// The thumbnail is a once-per-segment side effect. A newer trigger frame
		// may supersede its pixels before a worker starts, but it must inherit the
		// target or thumbnailPending would suppress every later attempt.
		if task.thumbnail == "" {
			task.thumbnail = previous.thumbnail
		}
		recordMemory.release(previous.bytes)
	}
	p.pending[rec] = task
	if !p.queued[rec] {
		p.queued[rec] = true
		p.queue = append(p.queue, rec)
		p.cond.Signal()
	}
	p.mu.Unlock()
}

func (p *snapshotPool) cancel(rec *Recorder) {
	p.mu.Lock()
	if task, ok := p.pending[rec]; ok {
		delete(p.pending, rec)
		recordMemory.release(task.bytes)
	}
	p.mu.Unlock()
}

func (p *snapshotPool) run() {
	for {
		p.mu.Lock()
		for len(p.queue) == 0 {
			p.cond.Wait()
		}
		rec := p.queue[0]
		p.queue = p.queue[1:]
		p.queued[rec] = false
		task, ok := p.pending[rec]
		delete(p.pending, rec)
		p.mu.Unlock()
		if !ok {
			continue
		}

		rec.processSnapshot(task)
		recordMemory.release(task.bytes)
	}
}

func (r *Recorder) processSnapshot(task snapshotTask) {
	ctx, cancel := context.WithTimeout(r.snapshotContext(), snapshotDeadline)
	defer cancel()

	var jpeg []byte
	switch task.codec {
	case core.CodecH264, core.CodecH265:
		var err error
		jpeg, err = ffmpeg.JPEGWithScaleContext(ctx, annexb.DecodeAVCC(task.payload, true), 640, -1)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			log.Debug().Err(err).Str("src", r.src).Msg("[record] keyframe jpeg transcode failed")
		}
	case core.CodecJPEG:
		jpeg = mjpeg.FixJPEG(task.payload)
	}
	if len(jpeg) == 0 || ctx.Err() != nil {
		return
	}

	r.snapshotMu.Lock()
	r.lastKey = jpeg
	r.lastKeyAt = task.at
	r.snapshotMu.Unlock()

	if task.thumbnail != "" {
		if err := writeThumbnail(task.thumbnail, jpeg); err != nil {
			log.Debug().Err(err).Str("src", r.src).Msg("[record] save thumbnail failed")
		}
	}
}

func writeThumbnail(recordingPath string, jpeg []byte) error {
	thumb := recordingPath[:len(recordingPath)-len(filepath.Ext(recordingPath))] + ".thumb"
	tmp, err := os.CreateTemp(filepath.Dir(thumb), ".thumb-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(jpeg); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, thumb)
}
