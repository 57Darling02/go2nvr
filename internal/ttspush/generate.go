package ttspush

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/difyz9/edge-tts-go/pkg/communicate"
)

type GenerateRequest struct {
	Text           string
	Voice          string
	Rate           string
	Pitch          string
	Volume         string
	Format         string
	Proxy          string
	ConnectTimeout int
	ReceiveTimeout int
	SampleRate     int
}

func GenerateStream(ctx context.Context, req GenerateRequest) (io.ReadCloser, error) {
	if err := req.normalize(); err != nil {
		return nil, err
	}

	mp3, err := synthesizeMP3Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	switch req.Format {
	case "mp3":
		return mp3, nil
	case "wav":
		return transcodeStream(ctx, mp3, req.SampleRate, "wav")
	case "pcm16le", "s16le":
		return transcodeStream(ctx, mp3, req.SampleRate, "pcm16le")
	default:
		_ = mp3.Close()
		return nil, errors.New("unsupported format: " + req.Format)
	}
}

func (r *GenerateRequest) normalize() error {
	r.Text = strings.TrimSpace(r.Text)
	if r.Text == "" {
		return errors.New("text is required")
	}
	if r.Voice == "" {
		r.Voice = defaultVoice
	}
	if r.Rate == "" {
		r.Rate = defaultRate
	}
	if r.Pitch == "" {
		r.Pitch = defaultPitch
	}
	if r.Volume == "" {
		r.Volume = defaultVolume
	}
	if r.ConnectTimeout <= 0 {
		r.ConnectTimeout = defaultConnectTimeout
	}
	if r.ReceiveTimeout <= 0 {
		r.ReceiveTimeout = defaultReceiveTimeout
	}
	if r.SampleRate <= 0 {
		r.SampleRate = defaultSampleRate
	}
	r.Format = strings.ToLower(strings.TrimSpace(r.Format))
	if r.Format == "" {
		r.Format = defaultGenerateFormat
	}
	return nil
}

func synthesizeMP3Stream(ctx context.Context, req GenerateRequest) (io.ReadCloser, error) {
	comm, err := communicate.NewCommunicate(
		req.Text,
		req.Voice,
		req.Rate,
		req.Volume,
		req.Pitch,
		req.Proxy,
		req.ConnectTimeout,
		req.ReceiveTimeout,
	)
	if err != nil {
		return nil, err
	}

	streamCtx, cancel := context.WithCancel(ctx)
	chunks, errs := comm.Stream(streamCtx)

	reader, writer := io.Pipe()

	go func() {
		defer cancel()

		hasAudio := false
		dropped := false

		for chunk := range chunks {
			if chunk.Type != "audio" || len(chunk.Data) == 0 {
				continue
			}
			hasAudio = true
			if dropped {
				continue
			}
			if _, werr := writer.Write(chunk.Data); werr != nil {
				dropped = true
				_ = writer.CloseWithError(werr)
			}
		}

		for streamErr := range errs {
			if dropped {
				continue
			}
			if streamErr != nil {
				_ = writer.CloseWithError(streamErr)
				return
			}
		}

		if dropped {
			// output already closed; chunks/errs were drained to avoid blocking producer
			return
		}

		if !hasAudio {
			_ = writer.CloseWithError(errors.New("no audio generated"))
			return
		}

		_ = writer.Close()
	}()

	return &cancelReadCloser{ReadCloser: reader, cancel: cancel}, nil
}

func transcodeStream(ctx context.Context, input io.ReadCloser, sampleRate int, outFormat string) (io.ReadCloser, error) {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-nostdin",
		"-i", "pipe:0",
		"-vn",
		"-ac", "1",
		"-ar", strconv.Itoa(sampleRate),
		"-c:a", "pcm_s16le",
	}

	switch outFormat {
	case "wav":
		args = append(args, "-f", "wav", "pipe:1")
	case "pcm16le":
		args = append(args, "-f", "s16le", "pipe:1")
	default:
		_ = input.Close()
		return nil, errors.New("unsupported transcode format: " + outFormat)
	}

	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, "ffmpeg", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		_ = input.Close()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = input.Close()
		return nil, err
	}

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err = cmd.Start(); err != nil {
		cancel()
		_ = input.Close()
		return nil, err
	}

	go func() {
		_, copyErr := io.Copy(stdin, input)
		_ = stdin.Close()
		_ = input.Close()
		if copyErr != nil {
			cancel()
		}
	}()

	outReader, outWriter := io.Pipe()
	go func() {
		_, copyErr := io.Copy(outWriter, stdout)
		waitErr := cmd.Wait()

		if copyErr != nil && !errors.Is(copyErr, io.ErrClosedPipe) {
			_ = outWriter.CloseWithError(copyErr)
			cancel()
			return
		}

		if waitErr != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg != "" {
				_ = outWriter.CloseWithError(fmt.Errorf("ffmpeg: %w: %s", waitErr, msg))
			} else {
				_ = outWriter.CloseWithError(fmt.Errorf("ffmpeg: %w", waitErr))
			}
			cancel()
			return
		}

		_ = outWriter.Close()
		cancel()
	}()

	return &cancelReadCloser{ReadCloser: outReader, cancel: cancel}, nil
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	c.cancel()
	return c.ReadCloser.Close()
}
