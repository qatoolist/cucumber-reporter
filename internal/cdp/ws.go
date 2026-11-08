// Package cdp is a minimal Chrome DevTools Protocol client over WebSocket,
// built on the standard library alone. It exists for the repository's
// headless browser checks (make browser-check) only and is not part of the
// public API.
package cdp

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6455 mandates SHA-1 for the handshake accept value.
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

	// maxMessage bounds a reassembled message; printToPDF replies run to
	// several MiB.
	maxMessage = 256 << 20

	opCont  = 0x0
	opText  = 0x1
	opBin   = 0x2
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xA
)

// errClosed is returned by readMessage once the peer sent a close frame.
var errClosed = errors.New("cdp: websocket closed by peer")

// wsConn is a client-side WebSocket connection (RFC 6455). One goroutine may
// call readMessage while others call writeText.
type wsConn struct {
	conn net.Conn
	br   *bufio.Reader
	wmu  sync.Mutex
}

// dialWS connects to a ws:// URL and completes the HTTP upgrade handshake.
func dialWS(ctx context.Context, rawURL string) (*wsConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("cdp: parse websocket url: %w", err)
	}
	if u.Scheme != "ws" {
		return nil, fmt.Errorf("cdp: unsupported websocket scheme %q", u.Scheme)
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("cdp: dial %s: %w", u.Host, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	c, err := handshake(ctx, conn, u)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return c, nil
}

func handshake(ctx context.Context, conn net.Conn, u *url.URL) (*wsConn, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("cdp: websocket nonce: %w", err)
	}
	key := base64.StdEncoding.EncodeToString(nonce[:])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+u.Host+u.RequestURI(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("cdp: build handshake: %w", err)
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")
	if err = req.Write(conn); err != nil {
		return nil, fmt.Errorf("cdp: write handshake: %w", err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, fmt.Errorf("cdp: read handshake: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("cdp: handshake status %s", resp.Status)
	}
	if !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") || !headerToken(resp.Header.Get("Connection"), "upgrade") {
		return nil, errors.New("cdp: invalid websocket upgrade headers")
	}
	sum := sha1.Sum([]byte(key + wsGUID)) //nolint:gosec // see import.
	if got, want := resp.Header.Get("Sec-WebSocket-Accept"), base64.StdEncoding.EncodeToString(sum[:]); got != want {
		return nil, fmt.Errorf("cdp: bad Sec-WebSocket-Accept %q", got)
	}
	return &wsConn{conn: conn, br: br}, nil
}

// writeText sends one masked, unfragmented text frame.
func (c *wsConn) writeText(payload []byte) error {
	return c.writeFrameContext(context.Background(), opText, payload)
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	return c.writeFrameContext(context.Background(), opcode, payload)
}

func (c *wsConn) writeTextContext(ctx context.Context, payload []byte) error {
	return c.writeFrameContext(ctx, opText, payload)
}

func (c *wsConn) writeFrameContext(ctx context.Context, opcode byte, payload []byte) error {
	hdr := make([]byte, 0, 14)
	hdr = append(hdr, 0x80|opcode)
	switch n := len(payload); {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n <= 0xffff:
		hdr = append(hdr, 0x80|126)
		hdr = binary.BigEndian.AppendUint16(hdr, uint16(n))
	default:
		hdr = append(hdr, 0x80|127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return fmt.Errorf("cdp: websocket mask: %w", err)
	}
	hdr = append(hdr, mask[:]...)
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok {
		deadline = d
	}
	if e := c.conn.SetWriteDeadline(deadline); e != nil {
		return e
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = c.conn.SetWriteDeadline(time.Now()); close(done) })
	defer func() {
		if !stop() {
			<-done
		}
	}()
	frame := append(hdr, masked...)
	n, err := c.conn.Write(frame)
	if err == nil && n != len(frame) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return fmt.Errorf("cdp: websocket write: %w", err)
	}
	return nil
}

// readMessage returns the next complete data message, reassembling fragments
// and answering pings. It returns errClosed after a close frame.
func (c *wsConn) readMessage() ([]byte, error) {
	var msg []byte
	fragmented := false
	textFrame := false
	for {
		fin, opcode, payload, err := c.readFrame(len(msg))
		if err != nil {
			return nil, err
		}
		switch opcode {
		case opPing:
			if err := c.writeFrame(opPong, payload); err != nil {
				return nil, err
			}
		case opPong:
		case opClose:
			if len(payload) == 1 {
				return nil, errors.New("cdp: malformed close frame")
			}
			if len(payload) >= 2 {
				code := binary.BigEndian.Uint16(payload[:2])
				if code < 1000 || code >= 5000 || code == 1004 || code == 1005 || code == 1006 || code == 1015 || (code > 1014 && code < 3000) || !utf8.Valid(payload[2:]) {
					return nil, errors.New("cdp: invalid close status or reason")
				}
			}
			_ = c.writeFrame(opClose, nil)
			return nil, errClosed
		case opText, opBin, opCont:
			if opcode == opCont && !fragmented || opcode != opCont && fragmented {
				return nil, errors.New("cdp: invalid websocket continuation")
			}
			if opcode != opCont {
				textFrame = opcode == opText
			}
			fragmented = !fin
			msg = append(msg, payload...)
			if fin {
				if textFrame && !utf8.Valid(msg) {
					return nil, errors.New("cdp: invalid websocket UTF-8")
				}
				return msg, nil
			}
		default:
			return nil, fmt.Errorf("cdp: unexpected websocket opcode %#x", opcode)
		}
	}
}

func (c *wsConn) readFrame(have int) (fin bool, opcode byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.br, h[:]); err != nil {
		return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
	}
	fin, opcode = h[0]&0x80 != 0, h[0]&0x0f
	if h[0]&0x70 != 0 || h[1]&0x80 != 0 {
		return false, 0, nil, errors.New("cdp: invalid server frame flags")
	}
	if opcode >= 8 && (!fin || h[1]&0x7f > 125) {
		return false, 0, nil, errors.New("cdp: invalid control frame")
	}
	if opcode != opCont && opcode != opText && opcode != opBin && opcode != opClose && opcode != opPing && opcode != opPong {
		return false, 0, nil, errors.New("cdp: invalid opcode")
	}
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(c.br, b[:]); err != nil {
			return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
		if n < 126 {
			return false, 0, nil, errors.New("cdp: noncanonical frame length")
		}
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(c.br, b[:]); err != nil {
			return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
		}
		n = binary.BigEndian.Uint64(b[:])
		if n <= 65535 || n>>63 != 0 {
			return false, 0, nil, errors.New("cdp: noncanonical frame length")
		}
	}
	if n > maxMessage || n > uint64(maxMessage-have) { //nolint:gosec // have <= maxMessage by construction, so the subtraction is non-negative.
		return false, 0, nil, fmt.Errorf("cdp: websocket message exceeds %d bytes", maxMessage)
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.br, payload); err != nil {
		return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
	}

	return fin, opcode, payload, nil
}

// Close closes the underlying connection.
func (c *wsConn) Close() error { return c.conn.Close() }

func headerToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
