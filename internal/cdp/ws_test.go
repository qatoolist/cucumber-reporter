package cdp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // RFC 6455 mandates SHA-1 for the handshake accept value.
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const rfcGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// serverConn is the hand-written server side of a WebSocket connection.
type serverConn struct {
	conn net.Conn
	br   *bufio.Reader
}

func (s *serverConn) readFrame(t *testing.T) (opcode byte, payload []byte) {
	t.Helper()
	var h [2]byte
	if _, err := io.ReadFull(s.br, h[:]); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
			t.Errorf("server read header: %v", err)
		}
		return opClose, nil
	}
	if h[1]&0x80 == 0 {
		t.Errorf("client frame was not masked")
	}
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var b [2]byte
		_, _ = io.ReadFull(s.br, b[:])
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		_, _ = io.ReadFull(s.br, b[:])
		n = binary.BigEndian.Uint64(b[:])
	}
	var mask [4]byte
	_, _ = io.ReadFull(s.br, mask[:])
	payload = make([]byte, n)
	_, _ = io.ReadFull(s.br, payload)
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return h[0] & 0x0f, payload
}

func (s *serverConn) writeFrame(t *testing.T, fin bool, opcode byte, payload []byte) {
	t.Helper()
	b0 := opcode
	if fin {
		b0 |= 0x80
	}
	hdr := []byte{b0}
	switch n := len(payload); {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n <= 0xffff:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	if _, err := s.conn.Write(append(hdr, payload...)); err != nil {
		t.Errorf("server write: %v", err)
	}
}

func (s *serverConn) writeText(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	s.writeFrame(t, true, 0x1, b)
}

// newWSServer starts an httptest server that performs the WebSocket handshake
// by hand and then runs handler on the raw connection.
func newWSServer(t *testing.T, handler func(*serverConn)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("not a hijacker")
			return
		}
		conn, brw, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + rfcGUID)) //nolint:gosec // see import.
		resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n"
		if _, err := conn.Write([]byte(resp)); err != nil {
			t.Errorf("handshake write: %v", err)
			return
		}
		handler(&serverConn{conn: conn, br: brw.Reader})
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/devtools/page/X"
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestWSFramesRoundTrip(t *testing.T) {
	big := strings.Repeat("x", 70000)
	pong := make(chan string, 1)
	echoed := make(chan string, 3)
	url := newWSServer(t, func(s *serverConn) {
		s.writeFrame(t, true, 0x9, []byte("hi")) // ping
		op, p := s.readFrame(t)
		if op != 0xA {
			t.Errorf("expected pong, got opcode %x", op)
		}
		pong <- string(p)
		s.writeFrame(t, false, 0x1, []byte("frag"))
		s.writeFrame(t, false, 0x0, []byte("men"))
		s.writeFrame(t, true, 0x0, []byte("ted"))
		s.writeFrame(t, true, 0x1, []byte(big))
		for range 3 {
			_, p := s.readFrame(t)
			echoed <- string(p)
		}
		s.writeFrame(t, true, 0x8, nil)
	})
	c, err := dialWS(testCtx(t), url)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for i, want := range []string{"fragmented", big} {
		got, err := c.readMessage()
		if err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
		if string(got) != want {
			t.Fatalf("message %d: got %d bytes", i, len(got))
		}
	}
	// Client sends: short, 16-bit length, 64-bit length.
	mid := strings.Repeat("m", 300)
	for _, m := range []string{"a", mid, big} {
		if err := c.writeText([]byte(m)); err != nil {
			t.Fatal(err)
		}
	}
	if got := <-pong; got != "hi" {
		t.Fatalf("pong payload %q", got)
	}
	for _, want := range []string{"a", mid, big} {
		if got := <-echoed; got != want {
			t.Fatalf("server saw %d bytes, want %d", len(got), len(want))
		}
	}
	if _, err := c.readMessage(); err == nil {
		t.Fatal("expected error after close frame")
	}
}

func TestWSRejectsBadAccept(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		defer conn.Close()
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: bogus\r\n\r\n"))
	}))
	defer srv.Close()
	if _, err := dialWS(testCtx(t), "ws"+strings.TrimPrefix(srv.URL, "http")+"/x"); err == nil {
		t.Fatal("expected handshake failure")
	}
}

type req struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func TestCallOutOfOrderErrorAndEvents(t *testing.T) {
	url := newWSServer(t, func(s *serverConn) {
		var reqs []req
		for len(reqs) < 3 {
			_, p := s.readFrame(t)
			var r req
			if err := json.Unmarshal(p, &r); err != nil {
				t.Errorf("bad request: %v", err)
				return
			}
			reqs = append(reqs, r)
		}
		// Reply in reverse order; second request gets an error.
		s.writeText(t, map[string]any{"method": "Runtime.exceptionThrown", "params": map[string]any{
			"exceptionDetails": map[string]any{"text": "Uncaught", "exception": map[string]any{"description": "Error: boom"}},
		}})
		s.writeText(t, map[string]any{"id": reqs[2].ID, "result": map[string]any{"v": "third"}})
		s.writeText(t, map[string]any{"id": reqs[1].ID, "error": map[string]any{"code": -32000, "message": "nope"}})
		s.writeText(t, map[string]any{"id": reqs[0].ID, "result": map[string]any{"v": "first"}})
		_, _ = io.Copy(io.Discard, s.conn)
	})
	w, err := dialWS(testCtx(t), url)
	if err != nil {
		t.Fatal(err)
	}
	b := newBrowser(w)
	defer b.Close()

	type res struct{ V string }
	results := make([]res, 3)
	errs := make(chan error, 3)
	started := make(chan struct{})
	for i := range 3 {
		go func() {
			<-started
			errs <- b.Call(testCtx(t), "M"+string(rune('0'+i)), map[string]int{"i": i}, &results[i])
		}()
	}
	close(started)
	var failed int
	for range 3 {
		if err := <-errs; err != nil {
			failed++
			if !strings.Contains(err.Error(), "nope") {
				t.Errorf("unexpected error %v", err)
			}
		}
	}
	if failed != 1 {
		t.Fatalf("expected exactly one CDP error, got %d", failed)
	}
	// The exception event arrives before the replies, so it is recorded by now.
	ps := b.Problems()
	if len(ps) != 1 || !strings.Contains(ps[0], "Error: boom") {
		t.Fatalf("problems = %q", ps)
	}
}

func TestEvalAndEvents(t *testing.T) {
	url := newWSServer(t, func(s *serverConn) {
		for {
			op, p := s.readFrame(t)
			var r req
			if op == opClose || json.Unmarshal(p, &r) != nil {
				return
			}
			switch r.Method {
			case "Runtime.evaluate":
				var in struct {
					Expression                  string
					AwaitPromise, ReturnByValue bool
				}
				_ = json.Unmarshal(r.Params, &in)
				if !in.AwaitPromise || !in.ReturnByValue {
					t.Errorf("evaluate flags missing: %s", r.Params)
				}
				if in.Expression == "throw" {
					s.writeText(t, map[string]any{"id": r.ID, "result": map[string]any{
						"exceptionDetails": map[string]any{"text": "Uncaught", "exception": map[string]any{"description": "Error: bad"}},
					}})
					continue
				}
				s.writeText(t, map[string]any{"id": r.ID, "result": map[string]any{"result": map[string]any{"type": "number", "value": 42}}})
			case "Page.navigate":
				s.writeText(t, map[string]any{"id": r.ID, "result": map[string]any{"frameId": "F"}})
				s.writeText(t, map[string]any{"method": "Log.entryAdded", "params": map[string]any{"entry": map[string]any{"level": "error", "text": "CSP blocked", "url": "http://x"}}})
				s.writeText(t, map[string]any{"method": "Log.entryAdded", "params": map[string]any{"entry": map[string]any{"level": "info", "text": "fine"}}})
				s.writeText(t, map[string]any{"method": "Runtime.consoleAPICalled", "params": map[string]any{"type": "error", "args": []any{map[string]any{"type": "string", "value": "oops"}}}})
				s.writeText(t, map[string]any{"method": "Page.loadEventFired", "params": map[string]any{"timestamp": 1}})
			}
		}
	})
	w, err := dialWS(testCtx(t), url)
	if err != nil {
		t.Fatal(err)
	}
	b := newBrowser(w)
	defer b.Close()
	ctx := testCtx(t)

	var n int
	if err := b.Eval(ctx, "6*7", &n); err != nil || n != 42 {
		t.Fatalf("Eval = %d, %v", n, err)
	}
	if err := b.Eval(ctx, "throw", nil); err == nil || !strings.Contains(err.Error(), "Error: bad") {
		t.Fatalf("expected exception error, got %v", err)
	}
	if err := b.Navigate(ctx, "about:blank"); err != nil {
		t.Fatal(err)
	}
	ps := b.Problems()
	if len(ps) != 2 || !strings.Contains(ps[0], "CSP blocked") || !strings.Contains(ps[1], "oops") {
		t.Fatalf("problems = %q", ps)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := b.Call(cctx, "X", nil, nil); err == nil {
		t.Fatal("expected context error")
	}
}

func TestWSRejectsProtocolViolations(t *testing.T) {
	for _, frame := range [][]byte{{0xc1, 0}, {0x81, 0x80}, {0x09, 0}, {0x8b, 0}, {0x81, 126, 0, 1, 'x'}, {0x80, 0}, {0x81, 1, 0xff}, {0x88, 1, 0}, {0x88, 2, 3, 0xed}} {
		c := &wsConn{br: bufio.NewReader(bytes.NewReader(frame))}
		if _, err := c.readMessage(); err == nil {
			t.Fatalf("accepted malformed frame %x", frame)
		}
	}
	if headerToken("notupgrade", "upgrade") || !headerToken("keep-alive, Upgrade", "upgrade") {
		t.Fatal("invalid Connection token handling")
	}
}

func TestWSWriteHonorsDeadline(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	c := &wsConn{conn: client}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := c.writeTextContext(ctx, []byte("blocked")); err == nil || time.Since(start) > time.Second {
		t.Fatalf("write ignored deadline: %v", err)
	}
}
