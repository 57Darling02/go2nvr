package target

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ipwebcamQueueSize     = 16
	ipwebcamClientIdleTTL = 5 * time.Minute
	ipwebcamCleanupEvery  = time.Minute
	ipwebcamWriteTimeout  = 8 * time.Second
)

func PushIPWebcam(ctx context.Context, opts IPWebcamOptions, generate Generator) (int, error) {
	if generate == nil {
		return 0, errors.New("generate callback is nil")
	}

	opts.URL = strings.TrimSpace(opts.URL)
	if opts.URL == "" {
		return 0, errors.New("url is required")
	}
	if opts.SampleRate <= 0 {
		opts.SampleRate = 24000
	}
	if opts.ChunkMS <= 0 {
		opts.ChunkMS = 40
	}

	client := getIPWebcamClient(opts.URL, opts.InsecureTLS, opts.SampleRate)
	return client.pushPCM(ctx, opts.SampleRate, opts.ChunkMS, opts.Realtime, generate)
}

var ipwebcamClients sync.Map
var ipwebcamCleanerOnce sync.Once

func getIPWebcamClient(rawURL string, insecureTLS bool, sampleRate int) *ipwebcamClient {
	ipwebcamCleanerOnce.Do(startIPWebcamCleaner)

	key := rawURL + "|insecure=" + strconv.FormatBool(insecureTLS) + "|sample_rate=" + strconv.Itoa(sampleRate)
	if v, ok := ipwebcamClients.Load(key); ok {
		return v.(*ipwebcamClient)
	}

	client := newIPWebcamClient(rawURL, insecureTLS)
	v, loaded := ipwebcamClients.LoadOrStore(key, client)
	if loaded {
		client.close()
		return v.(*ipwebcamClient)
	}
	return client
}

func startIPWebcamCleaner() {
	go func() {
		ticker := time.NewTicker(ipwebcamCleanupEvery)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			ipwebcamClients.Range(func(k, v any) bool {
				client, ok := v.(*ipwebcamClient)
				if !ok {
					return true
				}

				if !client.shouldClose(now) {
					return true
				}

				ipwebcamClients.Delete(k)
				client.close()
				return true
			})
		}
	}()
}

func newIPWebcamClient(rawURL string, insecureTLS bool) *ipwebcamClient {
	client := &ipwebcamClient{
		url:         rawURL,
		insecureTLS: insecureTLS,
		queue:       make(chan pushRequest, ipwebcamQueueSize),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	client.touch()
	go client.run()
	return client
}

type ipwebcamClient struct {
	url         string
	insecureTLS bool

	queue chan pushRequest
	stop  chan struct{}
	done  chan struct{}

	closed   atomic.Bool
	closing  atomic.Bool
	lastUsed atomic.Int64
	inflight atomic.Int32

	closeOnce sync.Once

	conn       *websocket.Conn
	headerSent bool
}

func (c *ipwebcamClient) pushPCM(ctx context.Context, sampleRate int, chunkMS int, realtime bool, generate Generator) (int, error) {
	if c.closed.Load() || c.closing.Load() {
		return 0, errors.New("ipwebcam client is closed")
	}

	result := make(chan pushResult, 1)
	req := pushRequest{
		ctx:        ctx,
		sampleRate: sampleRate,
		chunkMS:    chunkMS,
		realtime:   realtime,
		generate:   generate,
		result:     result,
	}

	c.touch()

	select {
	case c.queue <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		return 0, errors.New("ipwebcam queue is full")
	}

	select {
	case out := <-result:
		return out.sent, out.err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (c *ipwebcamClient) run() {
	defer close(c.done)

	for {
		select {
		case req := <-c.queue:
			c.inflight.Add(1)
			c.touch()

			if err := req.ctx.Err(); err != nil {
				c.inflight.Add(-1)
				req.reply(pushResult{err: err})
				continue
			}

			sent, err := c.process(req.ctx, req.generate, req.sampleRate, req.chunkMS, req.realtime)

			c.inflight.Add(-1)
			c.touch()
			req.reply(pushResult{sent: sent, err: err})
		case <-c.stop:
			c.closeConn()
			for {
				select {
				case req := <-c.queue:
					req.reply(pushResult{err: errors.New("ipwebcam client is closed")})
				default:
					return
				}
			}
		}
	}
}

func (c *ipwebcamClient) process(ctx context.Context, generate Generator, sampleRate int, chunkMS int, realtime bool) (int, error) {
	stream, err := generate(ctx)
	if err != nil {
		return 0, err
	}
	defer stream.Close()

	if err := c.ensureConn(ctx); err != nil {
		return 0, err
	}

	if !c.headerSent {
		if err := c.writeBinary(wavStreamHeader(sampleRate)); err != nil {
			if err = c.reconnect(ctx); err != nil {
				return 0, err
			}
			if err = c.writeBinary(wavStreamHeader(sampleRate)); err != nil {
				return 0, err
			}
		}
		c.headerSent = true
	}

	chunkSize := sampleRate * 2 * chunkMS / 1000
	if chunkSize < 2 {
		chunkSize = 2
	}
	if chunkSize%2 != 0 {
		chunkSize++
	}
	bytesPerSecond := sampleRate * 2
	sent := 0
	buf := make([]byte, chunkSize)

	for {
		if err := ctx.Err(); err != nil {
			return sent, err
		}

		n, readErr := io.ReadFull(stream, buf)
		if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
			return sent, readErr
		}

		if n == 0 && errors.Is(readErr, io.EOF) {
			return sent, nil
		}

		frame := buf[:n]
		if len(frame)%2 != 0 {
			frame = frame[:len(frame)-1]
		}
		if len(frame) == 0 {
			if errors.Is(readErr, io.ErrUnexpectedEOF) {
				return sent, nil
			}
			continue
		}

		if err := c.writeBinary(frame); err != nil {
			if err = c.reconnect(ctx); err != nil {
				return sent, err
			}
			if err = c.writeBinary(wavStreamHeader(sampleRate)); err != nil {
				return sent, err
			}
			c.headerSent = true
			if err = c.writeBinary(frame); err != nil {
				return sent, err
			}
		}

		sent += len(frame)

		if realtime && bytesPerSecond > 0 {
			delay := time.Duration(len(frame)) * time.Second / time.Duration(bytesPerSecond)
			if delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return sent, ctx.Err()
				case <-timer.C:
				}
			}
		}

		if errors.Is(readErr, io.ErrUnexpectedEOF) || errors.Is(readErr, io.EOF) {
			return sent, nil
		}
	}
}

func (c *ipwebcamClient) ensureConn(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}
	return c.reconnect(ctx)
}

func (c *ipwebcamClient) reconnect(ctx context.Context) error {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}

	conn, err := dialWebSocket(ctx, c.url, c.insecureTLS)
	if err != nil {
		return err
	}

	c.conn = conn
	c.headerSent = false
	return nil
}

func (c *ipwebcamClient) writeBinary(payload []byte) error {
	if c.conn == nil {
		return errors.New("websocket is not connected")
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(ipwebcamWriteTimeout))
	return c.conn.WriteMessage(websocket.BinaryMessage, payload)
}

func (c *ipwebcamClient) closeConn() {
	if c.conn == nil {
		return
	}
	_ = c.conn.Close()
	c.conn = nil
	c.headerSent = false
}

func (c *ipwebcamClient) close() {
	if !c.closing.CompareAndSwap(false, true) {
		return
	}
	c.closeOnce.Do(func() {
		close(c.stop)
		<-c.done
		c.closed.Store(true)
	})
}

func (c *ipwebcamClient) touch() {
	c.lastUsed.Store(time.Now().UnixNano())
}

func (c *ipwebcamClient) shouldClose(now time.Time) bool {
	if c.closing.Load() || c.closed.Load() {
		return false
	}
	if len(c.queue) != 0 || c.inflight.Load() != 0 {
		return false
	}
	last := c.lastUsed.Load()
	if last <= 0 {
		return false
	}
	return now.Sub(time.Unix(0, last)) >= ipwebcamClientIdleTTL
}

type pushRequest struct {
	ctx        context.Context
	sampleRate int
	chunkMS    int
	realtime   bool
	generate   Generator
	result     chan pushResult
}

func (r pushRequest) reply(res pushResult) {
	select {
	case r.result <- res:
	default:
	}
}

type pushResult struct {
	sent int
	err  error
}

func dialWebSocket(ctx context.Context, rawURL string, insecureTLS bool) (*websocket.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	withoutUser := *u
	withoutUser.User = nil
	dialURL := withoutUser.String()

	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		header.Set("Authorization", "Basic "+token)
	}

	dialer := newWebSocketDialer(insecureTLS)

	conn, res, err := dialer.DialContext(ctx, dialURL, header)
	if err == nil {
		return conn, nil
	}
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}

	if username == "" || res == nil || res.StatusCode != http.StatusUnauthorized {
		return nil, err
	}

	digestHeader, derr := buildDigestAuthHeader(res.Header, username, password, http.MethodGet, withoutUser.RequestURI())
	if derr != nil {
		return nil, err
	}

	header.Set("Authorization", digestHeader)
	conn, _, err = dialer.DialContext(ctx, dialURL, header)
	return conn, err
}

func newWebSocketDialer(insecureTLS bool) *websocket.Dialer {
	d := *websocket.DefaultDialer
	if !insecureTLS {
		return &d
	}

	if d.TLSClientConfig == nil {
		d.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		return &d
	}

	cfg := d.TLSClientConfig.Clone()
	cfg.InsecureSkipVerify = true //nolint:gosec
	d.TLSClientConfig = cfg
	return &d
}

func wavStreamHeader(sampleRate int) []byte {
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], 0xFFFFFFFF)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], 0xFFFFFFFF)
	return header
}

func buildDigestAuthHeader(h http.Header, username, password, method, uri string) (string, error) {
	challenge := findDigestChallenge(h.Values("Www-Authenticate"))
	if challenge == "" {
		return "", errors.New("digest challenge not found")
	}

	params := parseDigestParams(challenge)
	realm := params["realm"]
	nonce := params["nonce"]
	if realm == "" || nonce == "" {
		return "", errors.New("digest challenge missing realm or nonce")
	}

	algorithm := strings.ToLower(params["algorithm"])
	if algorithm == "" {
		algorithm = "md5"
	}

	qop := chooseDigestQOP(params["qop"])
	cnonce := randomHex(8)
	nc := "00000001"

	ha1 := md5Hex(username + ":" + realm + ":" + password)
	if algorithm == "md5-sess" {
		ha1 = md5Hex(ha1 + ":" + nonce + ":" + cnonce)
	}

	ha2 := md5Hex(method + ":" + uri)

	var response string
	if qop != "" {
		response = md5Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
	} else {
		response = md5Hex(ha1 + ":" + nonce + ":" + ha2)
	}

	parts := []string{
		`Digest username="` + escapeQuoted(username) + `"`,
		`realm="` + escapeQuoted(realm) + `"`,
		`nonce="` + escapeQuoted(nonce) + `"`,
		`uri="` + escapeQuoted(uri) + `"`,
		`response="` + response + `"`,
	}

	if algorithm != "" {
		parts = append(parts, "algorithm="+strings.ToUpper(algorithm))
	}
	if opaque := params["opaque"]; opaque != "" {
		parts = append(parts, `opaque="`+escapeQuoted(opaque)+`"`)
	}
	if qop != "" {
		parts = append(parts, "qop="+qop, "nc="+nc, `cnonce="`+cnonce+`"`)
	}

	return strings.Join(parts, ", "), nil
}

func findDigestChallenge(values []string) string {
	for _, v := range values {
		s := strings.TrimSpace(v)
		if len(s) >= 6 && strings.EqualFold(s[:6], "digest") {
			return strings.TrimSpace(s[6:])
		}
		if i := strings.Index(strings.ToLower(s), "digest "); i >= 0 {
			return strings.TrimSpace(s[i+7:])
		}
	}
	return ""
}

func parseDigestParams(s string) map[string]string {
	params := make(map[string]string)

	var (
		i   int
		n   = len(s)
		key string
	)

	for i < n {
		for i < n && (s[i] == ' ' || s[i] == ',') {
			i++
		}
		start := i
		for i < n && s[i] != '=' && s[i] != ',' {
			i++
		}
		if i >= n || s[i] != '=' {
			break
		}
		key = strings.ToLower(strings.TrimSpace(s[start:i]))
		i++
		if i >= n {
			break
		}

		var val string
		if s[i] == '"' {
			i++
			start = i
			for i < n {
				if s[i] == '"' && s[i-1] != '\\' {
					break
				}
				i++
			}
			val = s[start:i]
			if i < n && s[i] == '"' {
				i++
			}
		} else {
			start = i
			for i < n && s[i] != ',' {
				i++
			}
			val = strings.TrimSpace(s[start:i])
		}

		if key != "" {
			params[key] = val
		}
	}

	return params
}

func chooseDigestQOP(raw string) string {
	if raw == "" {
		return ""
	}
	options := strings.Split(strings.ToLower(raw), ",")
	for _, opt := range options {
		v := strings.TrimSpace(opt)
		if v == "auth" {
			return v
		}
	}
	for _, opt := range options {
		v := strings.TrimSpace(opt)
		if v != "" {
			return v
		}
	}
	return ""
}

func randomHex(size int) string {
	if size <= 0 {
		size = 8
	}
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", sum[:])
}

func escapeQuoted(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}
