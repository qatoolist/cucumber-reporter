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
	"sync"
	"time"
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
	sum := sha1.Sum([]byte(key + wsGUID)) //nolint:gosec // see import.
	if got, want := resp.Header.Get("Sec-WebSocket-Accept"), base64.StdEncoding.EncodeToString(sum[:]); got != want {
		return nil, fmt.Errorf("cdp: bad Sec-WebSocket-Accept %q", got)
	}
	return &wsConn{conn: conn, br: br}, nil
}

// writeText sends one masked, unfragmented text frame.
func (c *wsConn) writeText(payload []byte) error {
	return c.writeFrame(opText, payload)
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
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
	if _, err := c.conn.Write(append(hdr, masked...)); err != nil {
		return fmt.Errorf("cdp: websocket write: %w", err)
	}
	return nil
}

// readMessage returns the next complete data message, reassembling fragments
// and answering pings. It returns errClosed after a close frame.
func (c *wsConn) readMessage() ([]byte, error) {
	var msg []byte
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
			_ = c.writeFrame(opClose, nil)
			return nil, errClosed
		case opText, opBin, opCont:
			msg = append(msg, payload...)
			if fin {
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
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(c.br, b[:]); err != nil {
			return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(c.br, b[:]); err != nil {
			return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > maxMessage || n > uint64(maxMessage-have) { //nolint:gosec // have <= maxMessage by construction, so the subtraction is non-negative.
		return false, 0, nil, fmt.Errorf("cdp: websocket message exceeds %d bytes", maxMessage)
	}
	var mask [4]byte
	masked := h[1]&0x80 != 0
	if masked {
		if _, err = io.ReadFull(c.br, mask[:]); err != nil {
			return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.br, payload); err != nil {
		return false, 0, nil, fmt.Errorf("cdp: websocket read: %w", err)
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return fin, opcode, payload, nil
}

// Close closes the underlying connection.
func (c *wsConn) Close() error { return c.conn.Close() }
