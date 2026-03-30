package sessionlogcollector

import (
	"bufio"
	"context"
	"os"
	"time"
)

const defaultPollInterval = 500 * time.Millisecond

// Tailer tails a JSONL-style log file and invokes OnLine for each complete line.
// It waits for the file to appear, handles partial lines across polls, and exits
// when the context is cancelled.
type Tailer struct {
	path         string
	pollInterval time.Duration
	onLine       func(line []byte)
	seekToEnd    bool // if true, skip existing content and only process new lines
}

// New creates a Tailer for the given path and line callback.
func New(path string, onLine func(line []byte)) *Tailer {
	return &Tailer{
		path:         path,
		pollInterval: defaultPollInterval,
		onLine:       onLine,
	}
}

// NewTailOnly creates a Tailer that skips existing file content and only
// processes lines appended after the tailer starts. Used after relaunch
// to avoid replaying old events from a moved session log.
func NewTailOnly(path string, onLine func(line []byte)) *Tailer {
	return &Tailer{
		path:         path,
		pollInterval: defaultPollInterval,
		onLine:       onLine,
		seekToEnd:    true,
	}
}

// Run starts tailing until ctx is cancelled.
func (t *Tailer) Run(ctx context.Context) {
	if t.onLine == nil {
		return
	}

	// Wait for the file to appear.
	var f *os.File
	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()
	for {
		var err error
		f, err = os.Open(t.path)
		if err == nil {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
	defer f.Close()

	if t.seekToEnd {
		f.Seek(0, 2) // seek to end
	}

	reader := bufio.NewReader(f)
	var partial []byte
	for {
		// Try to read all available lines.
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				// Partial data (no trailing newline yet) — accumulate.
				partial = append(partial, line...)
				break
			}
			if len(partial) > 0 {
				line = append(partial, line...)
				partial = nil
			}
			t.onLine(line)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}
