package cdp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const pollInterval = 25 * time.Millisecond

// Browser is a connection to one page target of a headless Chrome.
type Browser struct {
	ws  *wsConn
	cmd *exec.Cmd // nil when constructed around an existing connection

	processDone chan error
	nextID      atomic.Int64

	mu       sync.Mutex
	pending  map[int64]chan response
	subs     map[string][]chan json.RawMessage
	problems []string
	readErr  error

	done      chan struct{}
	closeOnce sync.Once
}

type response struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data"`
	} `json:"error"`
}

type message struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	response
}

func newBrowser(ws *wsConn) *Browser {
	b := &Browser{
		ws:      ws,
		pending: map[int64]chan response{},
		subs:    map[string][]chan json.RawMessage{},
		done:    make(chan struct{}),
	}
	go b.readLoop()
	return b
}

// FindChrome locates a Chrome-family executable: $CHROME first, then the
// macOS application bundles, then well-known names on PATH.
func FindChrome() (string, error) {
	if p := os.Getenv("CHROME"); p != "" {
		return p, nil
	}
	paths := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	}
	if home, e := os.UserHomeDir(); e == nil {
		paths = append(paths, filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"))
	}
	for _, base := range []string{os.Getenv("PROGRAMFILES"), os.Getenv("PROGRAMFILES(X86)"), os.Getenv("LOCALAPPDATA")} {
		if base != "" {
			for _, sub := range []string{"Google/Chrome/Application/chrome.exe", "Microsoft/Edge/Application/msedge.exe"} {
				paths = append(paths, filepath.Join(base, filepath.FromSlash(sub)))
			}
		}
	}
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() { //nolint:gosec // G703: operator-selected local browser locations, never report input.
			return p, nil
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", errors.New("cdp: no Chrome found; set CHROME to a Chrome, Chromium or Edge executable")
}

// Launch starts a headless Chrome with a fresh debugging port, connects to its
// first page target and enables the Page, Runtime and Log domains. The process
// is bound to ctx: canceling ctx kills it. Call Close when done.
func Launch(ctx context.Context, chromePath, userDataDir string) (*Browser, error) {
	portFile := filepath.Join(userDataDir, "DevToolsActivePort")
	if err := os.Remove(portFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("cdp: remove stale port file: %w", err)
	}
	cmd := exec.CommandContext(ctx, chromePath, //nolint:gosec // the path is the operator's chosen browser.
		"--headless=new", "--remote-debugging-port=0", "--user-data-dir="+userDataDir,
		"--no-first-run", "--no-default-browser-check", "--disable-background-networking",
		"--disable-gpu", "about:blank")
	if flags := os.Getenv("CHROME_FLAGS"); flags != "" {
		cmd.Args = append(cmd.Args[:len(cmd.Args)-1], append(strings.Fields(flags), "about:blank")...)
	}
	configureBrowserProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	var stderr browserOutput
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("cdp: start %s: %w", chromePath, err)
	}
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() { done <- cmd.Wait(); close(exited) }()
	kill := func() error {
		_ = cmd.Cancel()
		return <-done
	}
	startup, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	defer cancelStartup()
	go func() {
		select {
		case <-exited:
			cancelStartup()
		case <-startup.Done():
		}
	}()
	port, err := readPort(startup, portFile)
	if err != nil {
		processErr := kill()
		return nil, fmt.Errorf("cdp: Chrome startup failed: %w; stderr: %s", errors.Join(err, processErr), stderr.String())
	}
	wsURL, err := pageTarget(startup, port)
	if err != nil {
		_ = kill()
		return nil, fmt.Errorf("cdp: Chrome startup: %w; stderr: %s", err, stderr.String())
	}
	ws, err := dialWS(startup, wsURL)
	if err != nil {
		_ = kill()
		return nil, fmt.Errorf("cdp: Chrome startup: %w; stderr: %s", err, stderr.String())
	}
	b := newBrowser(ws)
	b.cmd = cmd
	b.processDone = done
	for _, m := range []string{"Page.enable", "Runtime.enable", "Log.enable"} {
		if err := b.Call(startup, m, nil, nil); err != nil {
			_ = b.Close()
			return nil, err
		}
	}
	if err := b.Call(startup, "Emulation.setFocusEmulationEnabled", map[string]any{"enabled": true}, nil); err != nil {
		_ = b.Close()
		return nil, err
	}
	return b, nil
}

func readPort(ctx context.Context, portFile string) (string, error) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		// Chrome writes the port then the browser path; wait for both lines so
		// a half-written file is never read.
		if data, err := os.ReadFile(portFile); err == nil { //nolint:gosec // path built from the caller's user data dir.
			if lines := strings.Split(string(data), "\n"); len(lines) >= 2 && lines[1] != "" {
				if _, err := strconv.Atoi(lines[0]); err == nil {
					return lines[0], nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("cdp: waiting for DevToolsActivePort: %w", ctx.Err())
		case <-t.C:
		}
	}
}

func pageTarget(ctx context.Context, port string) (string, error) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		u, err := fetchPageTarget(ctx, port)
		if err == nil && u != "" {
			return u, nil
		}
		select {
		case <-ctx.Done():
			if err == nil {
				err = errors.New("no page target")
			}
			return "", fmt.Errorf("cdp: listing targets: %w (%w)", err, ctx.Err())
		case <-t.C:
		}
	}
}

func fetchPageTarget(ctx context.Context, port string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/json/list", http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var targets []struct {
		Type string `json:"type"`
		WS   string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &targets); err != nil {
		return "", err
	}
	for _, t := range targets {
		if t.Type == "page" && t.WS != "" {
			return t.WS, nil
		}
	}
	return "", nil
}

// Close closes the socket and, for a launched browser, kills and reaps the
// process. It is safe to call more than once.
func (b *Browser) Close() error {
	var err error
	b.closeOnce.Do(func() {
		err = b.ws.Close()
		if b.cmd != nil {
			_ = b.cmd.Cancel()
			<-b.processDone //nolint:errcheck // Close is best-effort process cleanup.
		}
	})
	return err
}

func (b *Browser) readLoop() {
	defer close(b.done)
	for {
		data, err := b.ws.readMessage()
		if err != nil {
			b.mu.Lock()
			b.readErr = err
			for id, ch := range b.pending {
				delete(b.pending, id)
				close(ch)
			}
			b.mu.Unlock()
			return
		}
		var m message
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.Method != "" {
			b.dispatchEvent(m.Method, m.Params)
			continue
		}
		b.mu.Lock()
		ch, ok := b.pending[m.ID]
		delete(b.pending, m.ID)
		b.mu.Unlock()
		if ok {
			ch <- m.response
		}
	}
}

func (b *Browser) dispatchEvent(method string, params json.RawMessage) {
	if p := problemFrom(method, params); p != "" {
		b.mu.Lock()
		b.problems = append(b.problems, p)
		b.mu.Unlock()
	}
	b.mu.Lock()
	subs := append([]chan json.RawMessage(nil), b.subs[method]...)
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- params:
		default:
		}
	}
}

func problemFrom(method string, params json.RawMessage) string {
	switch method {
	case "Runtime.exceptionThrown":
		var p struct {
			ExceptionDetails exceptionDetails `json:"exceptionDetails"`
		}
		if json.Unmarshal(params, &p) == nil {
			return "exception: " + p.ExceptionDetails.String()
		}
	case "Log.entryAdded":
		var p struct {
			Entry struct{ Level, Text, URL string } `json:"entry"`
		}
		if json.Unmarshal(params, &p) == nil && p.Entry.Level == "error" {
			s := "log error: " + p.Entry.Text
			if p.Entry.URL != "" {
				s += " (" + p.Entry.URL + ")"
			}
			return s
		}
	case "Runtime.consoleAPICalled":
		var p struct {
			Type string `json:"type"`
			Args []struct {
				Value       any    `json:"value"`
				Description string `json:"description"`
			} `json:"args"`
		}
		if json.Unmarshal(params, &p) == nil && p.Type == "error" {
			parts := make([]string, 0, len(p.Args))
			for _, a := range p.Args {
				if a.Value != nil {
					parts = append(parts, fmt.Sprint(a.Value))
				} else {
					parts = append(parts, a.Description)
				}
			}
			return "console.error: " + strings.Join(parts, " ")
		}
	}
	return ""
}

type exceptionDetails struct {
	Text      string `json:"text"`
	Exception *struct {
		Description string `json:"description"`
	} `json:"exception"`
}

func (e exceptionDetails) String() string {
	if e.Exception != nil && e.Exception.Description != "" {
		return e.Exception.Description
	}
	return e.Text
}

// Problems returns a snapshot of everything the page reported as wrong:
// uncaught exceptions, error-level log entries (including CSP violations) and
// console.error calls.
func (b *Browser) Problems() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.problems...)
}

// subscribe returns a channel receiving the params of each method event and a
// cancel func.
func (b *Browser) subscribe(method string) (events <-chan json.RawMessage, cancel func()) {
	ch := make(chan json.RawMessage, 16)
	b.mu.Lock()
	b.subs[method] = append(b.subs[method], ch)
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subs[method]
		for i, c := range subs {
			if c == ch {
				b.subs[method] = append(subs[:i:i], subs[i+1:]...)
				return
			}
		}
	}
}

// Call performs one JSON-RPC request and unmarshals the result into result
// (which may be nil). A CDP error reply becomes a Go error. Safe for
// concurrent use.
func (b *Browser) Call(ctx context.Context, method string, params, result any) error {
	id := b.nextID.Add(1)
	payload := map[string]any{"id": id, "method": method}
	if params != nil {
		payload["params"] = params
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("cdp: marshal %s: %w", method, err)
	}
	ch := make(chan response, 1)
	b.mu.Lock()
	if b.readErr != nil {
		err := b.readErr
		b.mu.Unlock()
		return fmt.Errorf("cdp: %s: connection closed: %w", method, err)
	}
	b.pending[id] = ch
	b.mu.Unlock()
	forget := func() {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
	}
	if err := b.ws.writeTextContext(ctx, data); err != nil {
		forget()
		return err
	}
	select {
	case <-ctx.Done():
		forget()
		return fmt.Errorf("cdp: %s: %w", method, ctx.Err())
	case r, ok := <-ch:
		if !ok {
			b.mu.Lock()
			err := b.readErr
			b.mu.Unlock()
			return fmt.Errorf("cdp: %s: connection closed: %w", method, err)
		}
		if r.Error != nil {
			return fmt.Errorf("cdp: %s: %s (code %d) %s", method, r.Error.Message, r.Error.Code, r.Error.Data)
		}
		if result != nil && len(r.Result) > 0 {
			if err := json.Unmarshal(r.Result, result); err != nil {
				return fmt.Errorf("cdp: %s: decode result: %w", method, err)
			}
		}
		return nil
	}
}

// Eval evaluates a JavaScript expression (awaiting promises) and unmarshals
// the by-value result into out, which may be nil. An exception becomes an
// error carrying its description.
func (b *Browser) Eval(ctx context.Context, expr string, out any) error {
	var r struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *exceptionDetails `json:"exceptionDetails"`
	}
	err := b.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression": expr, "awaitPromise": true, "returnByValue": true,
	}, &r)
	if err != nil {
		return err
	}
	if r.ExceptionDetails != nil {
		return fmt.Errorf("cdp: evaluate: %s", r.ExceptionDetails)
	}
	if out != nil && len(r.Result.Value) > 0 {
		if err := json.Unmarshal(r.Result.Value, out); err != nil {
			return fmt.Errorf("cdp: evaluate: decode value: %w", err)
		}
	}
	return nil
}

// Navigate loads url and waits for the load event.
func (b *Browser) Navigate(ctx context.Context, url string) error {
	return b.untilLoad(ctx, "Page.navigate", map[string]any{"url": url})
}

// Reload reloads the page and waits for the load event.
func (b *Browser) Reload(ctx context.Context) error {
	return b.untilLoad(ctx, "Page.reload", nil)
}

func (b *Browser) untilLoad(ctx context.Context, method string, params any) error {
	loaded, cancel := b.subscribe("Page.loadEventFired")
	defer cancel()
	var r struct {
		ErrorText string `json:"errorText"`
	}
	if err := b.Call(ctx, method, params, &r); err != nil {
		return err
	}
	if r.ErrorText != "" {
		return fmt.Errorf("cdp: %s: %s", method, r.ErrorText)
	}
	select {
	case <-loaded:
		return nil
	case <-b.done:
		return fmt.Errorf("cdp: %s: connection closed before load", method)
	case <-ctx.Done():
		return fmt.Errorf("cdp: %s: waiting for load: %w", method, ctx.Err())
	}
}

// SetViewport emulates a viewport of w by h CSS pixels; widths under 600 are
// treated as mobile.
func (b *Browser) SetViewport(ctx context.Context, w, h int) error {
	return b.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width": w, "height": h, "deviceScaleFactor": 1, "mobile": w < 600,
	}, nil)
}

// Key presses and releases a key. modifiers is the CDP bit mask (1 Alt,
// 2 Ctrl, 4 Meta, 8 Shift). A single printable character without modifiers
// also produces text input.
func (b *Browser) Key(ctx context.Context, key string, modifiers int) error {
	down := map[string]any{"type": "keyDown", "key": key, "modifiers": modifiers}
	if len([]rune(key)) == 1 && modifiers == 0 {
		down["text"] = key
	}
	switch key {
	case "Tab":
		down["windowsVirtualKeyCode"] = 9
	case "Enter":
		down["windowsVirtualKeyCode"] = 13
	case "Escape":
		down["windowsVirtualKeyCode"] = 27
	case "ArrowDown":
		down["windowsVirtualKeyCode"] = 40
	case "ArrowUp":
		down["windowsVirtualKeyCode"] = 38
	}
	if err := b.Call(ctx, "Input.dispatchKeyEvent", down, nil); err != nil {
		return err
	}
	return b.Call(ctx, "Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": key, "modifiers": modifiers}, nil)
}

// InsertText inserts s into the focused element as if pasted.
func (b *Browser) InsertText(ctx context.Context, s string) error {
	return b.Call(ctx, "Input.insertText", map[string]any{"text": s}, nil)
}

// PrintPDF renders the page to PDF with backgrounds and the page's own CSS
// page size.
func (b *Browser) PrintPDF(ctx context.Context) ([]byte, error) {
	var r struct {
		Data string `json:"data"`
	}
	if err := b.Call(ctx, "Page.printToPDF", map[string]any{"printBackground": true, "preferCSSPageSize": true}, &r); err != nil {
		return nil, err
	}
	pdf, err := base64.StdEncoding.DecodeString(r.Data)
	if err != nil {
		return nil, fmt.Errorf("cdp: decode pdf: %w", err)
	}
	return pdf, nil
}

type browserOutput struct {
	mu   sync.Mutex
	data []byte
}

func (b *browserOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if n > 4096 {
		p = p[n-4096:]
	}
	b.data = append(b.data, p...)
	if len(b.data) > 4096 {
		b.data = b.data[len(b.data)-4096:]
	}
	return n, nil
}
func (b *browserOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }
