package target

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/AlexxIT/go2rtc/internal/streams"
	"github.com/AlexxIT/go2rtc/pkg/magic"
)

func PushBackchannel(ctx context.Context, opts BackchannelOptions, generate Generator) (int, error) {
	if generate == nil {
		return 0, errors.New("generate callback is nil")
	}

	dst := strings.TrimSpace(opts.Dst)
	if dst == "" {
		return 0, errors.New("dst is required")
	}

	targetStream := streams.Get(dst)
	if targetStream == nil {
		return 0, errors.New("stream not found: " + dst)
	}

	stream, err := generate(ctx)
	if err != nil {
		return 0, err
	}

	counter := newCountingReadCloser(stream)

	prod, err := magic.Open(counter)
	if err != nil {
		_ = counter.Close()
		return 0, err
	}

	if err = targetStream.Play(prod); err != nil {
		_ = counter.Close()
		return 0, err
	}

	if err = counter.Wait(ctx); err != nil {
		_ = counter.Close()
		return int(counter.Bytes()), err
	}

	_ = counter.Close()
	return int(counter.Bytes()), nil
}

type countingReadCloser struct {
	rc io.ReadCloser

	bytes atomic.Int64

	done    chan struct{}
	doneErr error
	doneMu  sync.Mutex
	doneOne sync.Once
}

func newCountingReadCloser(rc io.ReadCloser) *countingReadCloser {
	return &countingReadCloser{
		rc:   rc,
		done: make(chan struct{}),
	}
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	if n > 0 {
		c.bytes.Add(int64(n))
	}
	if err != nil {
		if errors.Is(err, io.EOF) {
			c.finish(nil)
		} else {
			c.finish(err)
		}
	}
	return n, err
}

func (c *countingReadCloser) Close() error {
	err := c.rc.Close()
	c.finish(nil)
	return err
}

func (c *countingReadCloser) Wait(ctx context.Context) error {
	select {
	case <-c.done:
		c.doneMu.Lock()
		err := c.doneErr
		c.doneMu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *countingReadCloser) Bytes() int64 {
	return c.bytes.Load()
}

func (c *countingReadCloser) finish(err error) {
	c.doneOne.Do(func() {
		c.doneMu.Lock()
		c.doneErr = err
		c.doneMu.Unlock()
		close(c.done)
	})
}
