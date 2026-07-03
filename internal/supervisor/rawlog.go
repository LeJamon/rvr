package supervisor

import (
	"os"
	"sync"
)

// cappedFile is an append-only log bounded to cap bytes. When the cap is
// reached it truncates and restarts from the top — a crude rotation that
// bounds disk use while keeping the most recent output (SPEC.md §4). The ring
// buffer, not this file, is the source of truth for attach replay.
type cappedFile struct {
	mu      sync.Mutex
	f       *os.File
	cap     int64
	written int64
}

func newCappedFile(path string, cap int64) (*cappedFile, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	return &cappedFile{f: f, cap: cap}, nil
}

func (c *cappedFile) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.f == nil {
		return len(p), nil
	}
	if c.written+int64(len(p)) > c.cap {
		if err := c.f.Truncate(0); err == nil {
			_, _ = c.f.Seek(0, 0)
			c.written = 0
		}
	}
	n, err := c.f.Write(p)
	c.written += int64(n)
	return n, err
}

func (c *cappedFile) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.f == nil {
		return nil
	}
	err := c.f.Close()
	c.f = nil
	return err
}
