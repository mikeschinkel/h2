package session

import (
	"encoding/json"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vito/midterm"

	"h2/internal/session/client"
	"h2/internal/session/message"
	"h2/internal/session/virtualterminal"
)

// newTestDaemon creates a Daemon with a properly initialized VT suitable
// for attach tests. No real child process is started.
func newTestDaemon() *Daemon {
	s := New("test", "true", nil)
	vt := &virtualterminal.VT{
		Rows: 24, Cols: 80, ChildRows: 22,
		Vt: midterm.NewTerminal(22, 80),
	}
	sb := midterm.NewTerminal(22, 80)
	sb.AutoResizeY = true
	sb.AppendOnly = true
	vt.Scrollback = sb
	s.VT = vt
	return &Daemon{Session: s, StartTime: time.Now()}
}

// bombWriter wraps an io.Writer and panics once armed. This avoids needing
// to swap the client's Output under the VT mutex during initial render.
type bombWriter struct {
	inner io.Writer
	armed atomic.Bool
}

func (b *bombWriter) Write(p []byte) (int, error) {
	if b.armed.Load() {
		panic("test-injected panic in client output")
	}
	return b.inner.Write(p)
}

// drainClient reads and discards all data from conn in a background goroutine.
func drainClient(conn net.Conn) {
	go io.Copy(io.Discard, conn)
}

// attachAndArm attaches to the daemon, waits for initial render to complete,
// then arms a bomb writer on all clients. Returns the client conn (already
// draining) and a done channel that closes when handleConn returns.
func attachAndArm(t *testing.T, d *Daemon, bomb *bombWriter) (clientConn net.Conn, done chan struct{}) {
	t.Helper()

	server, clientC := net.Pipe()
	done = make(chan struct{})

	go func() {
		defer close(done)
		d.handleConn(server)
	}()

	if err := message.SendRequest(clientC, &message.Request{
		Type: "attach",
		Cols: 80,
		Rows: 24,
	}); err != nil {
		t.Fatalf("send attach: %v", err)
	}
	resp, err := message.ReadResponse(clientC)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("response not OK: %s", resp.Error)
	}
	drainClient(clientC)

	// Wait for handleAttach to finish initial render and enter readClientInput.
	time.Sleep(50 * time.Millisecond)

	// Swap output to bomb writer under the mutex.
	d.Session.VT.Mu.Lock()
	d.Session.ForEachClient(func(cl *client.Client) {
		bomb.inner = cl.Output
		cl.Output = bomb
	})
	d.Session.VT.Mu.Unlock()

	bomb.armed.Store(true)
	return clientC, done
}

// sendResize sends a resize control frame with the given dimensions.
func sendResize(t *testing.T, conn net.Conn, cols, rows int) {
	t.Helper()
	ctrl, _ := json.Marshal(message.ResizeControl{
		Type: "resize",
		Cols: cols,
		Rows: rows,
	})
	if err := message.WriteFrame(conn, message.FrameTypeControl, ctrl); err != nil {
		t.Fatalf("write resize frame: %v", err)
	}
}

// TestAttach_PanicRecovery_CanReattach verifies that when a panic occurs
// inside readClientInput (e.g. during render), the daemon recovers,
// releases the mutex, and a new client can attach successfully.
func TestAttach_PanicRecovery_CanReattach(t *testing.T) {
	d := newTestDaemon()
	bomb := &bombWriter{}

	// First attach: trigger a panic.
	client1, done1 := attachAndArm(t, d, bomb)
	defer client1.Close()

	sendResize(t, client1, 60, 20)

	select {
	case <-done1:
	case <-time.After(5 * time.Second):
		t.Fatal("handleConn did not return after panic")
	}

	// Second attach: verify no deadlock.
	server2, client2 := net.Pipe()
	defer client2.Close()

	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		d.handleConn(server2)
	}()

	attached := make(chan struct{})
	go func() {
		if err := message.SendRequest(client2, &message.Request{
			Type: "attach",
			Cols: 80,
			Rows: 24,
		}); err != nil {
			return
		}
		resp, err := message.ReadResponse(client2)
		if err != nil || !resp.OK {
			return
		}
		drainClient(client2)
		close(attached)
	}()

	select {
	case <-attached:
		// Success — attached without deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("second attach deadlocked — mutex was not released after panic")
	}

	client2.Close()

	select {
	case <-done2:
	case <-time.After(5 * time.Second):
		t.Fatal("handleConn did not return after clean disconnect")
	}
}

// TestAttach_StatusAfterPanic verifies the daemon responds to status queries
// after an attach panic.
func TestAttach_StatusAfterPanic(t *testing.T) {
	d := newTestDaemon()
	bomb := &bombWriter{}

	client1, done1 := attachAndArm(t, d, bomb)
	defer client1.Close()

	sendResize(t, client1, 60, 20)

	select {
	case <-done1:
	case <-time.After(5 * time.Second):
		t.Fatal("handleConn did not return after panic")
	}

	// Status query should succeed.
	server2, client2 := net.Pipe()
	defer server2.Close()
	defer client2.Close()

	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		d.handleConn(server2)
	}()

	if err := message.SendRequest(client2, &message.Request{Type: "status"}); err != nil {
		t.Fatalf("send status: %v", err)
	}
	resp, err := message.ReadResponse(client2)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !resp.OK {
		t.Fatalf("status not OK: %s", resp.Error)
	}

	<-done2
}

// TestAttach_NarrowResize_NoPanic verifies that a resize to very small
// dimensions (cols < prompt length) does not panic.
func TestAttach_NarrowResize_NoPanic(t *testing.T) {
	d := newTestDaemon()

	server, clientC := net.Pipe()
	defer clientC.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.handleConn(server)
	}()

	if err := message.SendRequest(clientC, &message.Request{
		Type: "attach",
		Cols: 80,
		Rows: 24,
	}); err != nil {
		t.Fatalf("send attach: %v", err)
	}
	resp, err := message.ReadResponse(clientC)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("response not OK: %s", resp.Error)
	}
	drainClient(clientC)

	// Resize to cols=1 — would panic without the maxInput clamp.
	sendResize(t, clientC, 1, 5)

	// Send another normal resize to verify session is still alive.
	sendResize(t, clientC, 80, 24)

	// Send data to confirm the session processes input.
	if err := message.WriteFrame(clientC, message.FrameTypeData, []byte("hello")); err != nil {
		t.Fatalf("write data frame: %v", err)
	}

	clientC.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleConn did not return after disconnect")
	}
}

// TestAttach_NormalResize verifies normal resize works without issues.
func TestAttach_NormalResize(t *testing.T) {
	d := newTestDaemon()

	server, clientC := net.Pipe()
	defer clientC.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.handleConn(server)
	}()

	if err := message.SendRequest(clientC, &message.Request{
		Type: "attach",
		Cols: 80,
		Rows: 24,
	}); err != nil {
		t.Fatalf("send attach: %v", err)
	}
	resp, err := message.ReadResponse(clientC)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !resp.OK {
		t.Fatalf("response not OK: %s", resp.Error)
	}
	drainClient(clientC)

	sendResize(t, clientC, 120, 40)

	if err := message.WriteFrame(clientC, message.FrameTypeData, []byte("hello")); err != nil {
		t.Fatalf("write data frame: %v", err)
	}

	clientC.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleConn did not return after disconnect")
	}
}
