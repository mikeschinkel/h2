package session

import (
	"net"
	"testing"

	"h2/internal/config"
	"h2/internal/session/message"
	"h2/internal/session/virtualterminal"
)

func TestHandleStop_SetsQuitAndRespondsOK(t *testing.T) {
	s := NewFromConfig(&config.RuntimeConfig{
		AgentName:   "test",
		Command:     "true",
		HarnessType: "generic",
		SessionID:   "test-uuid",
		CWD:         "/tmp",
		StartedAt:   "2024-01-01T00:00:00Z",
	})
	s.VT = &virtualterminal.VT{} // minimal VT, no child process

	d := &Daemon{Session: s}

	// Create a connected socket pair.
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// Run handleStop in background.
	done := make(chan struct{})
	go func() {
		defer close(done)
		d.handleStop(server)
	}()

	// Read the response from the client side.
	resp, err := message.ReadResponse(client)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK response, got error: %s", resp.Error)
	}

	<-done

	if !s.Quit {
		t.Error("expected Session.Quit to be true after stop")
	}
}
