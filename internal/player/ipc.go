package player

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// EventKind describes a few mpv events worth surfacing to the UI.
type EventKind string

const (
	EventFileLoaded EventKind = "file-loaded"
	EventEnded      EventKind = "end-file"
	EventError      EventKind = "error"
	EventExited     EventKind = "exited"
)

// Event is a small, UI-friendly mpv notification.
type Event struct {
	Kind   EventKind
	Reason string
	Err    error
}

type rpcResult struct {
	data json.RawMessage
	err  error
}

type rpcMessage struct {
	RequestID int             `json:"request_id"`
	Error     string          `json:"error"`
	Data      json.RawMessage `json:"data"`
	Event     string          `json:"event"`
	Reason    string          `json:"reason"`
	FileError string          `json:"file_error"`
}

// ipc is a tiny JSON-RPC client for mpv's --input-ipc-server.
type ipc struct {
	conn   io.ReadWriteCloser
	events chan Event
	closed chan struct{}

	writeMu   sync.Mutex
	mu        sync.Mutex
	nextID    int
	pending   map[int]chan rpcResult
	closeOnce sync.Once
	readErr   error
}

func newIPC(conn io.ReadWriteCloser) *ipc {
	client := &ipc{
		conn:    conn,
		events:  make(chan Event, 64),
		closed:  make(chan struct{}),
		pending: make(map[int]chan rpcResult),
	}
	go client.readLoop()
	return client
}

func (c *ipc) Events() <-chan Event { return c.events }

func (c *ipc) Call(ctx context.Context, args ...any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	response := make(chan rpcResult, 1)
	c.pending[id] = response
	c.mu.Unlock()

	payload := map[string]any{
		"command":    args,
		"request_id": id,
	}
	if err := c.write(payload); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case result := <-response:
		return result.data, result.err
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-c.closed:
		return nil, c.connectionError()
	}
}

func (c *ipc) Notify(args ...any) error {
	return c.write(map[string]any{"command": args})
}

func (c *ipc) write(payload any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	select {
	case <-c.closed:
		return c.connectionError()
	default:
	}
	if err := json.NewEncoder(c.conn).Encode(payload); err != nil {
		c.fail(err)
		return err
	}
	return nil
}

func (c *ipc) readLoop() {
	defer close(c.closed)
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var message rpcMessage
		if err := json.Unmarshal(line, &message); err != nil {
			continue
		}
		if message.RequestID != 0 {
			c.mu.Lock()
			ch := c.pending[message.RequestID]
			delete(c.pending, message.RequestID)
			c.mu.Unlock()
			if ch != nil {
				result := rpcResult{data: message.Data}
				if message.Error != "" && message.Error != "success" {
					result.err = fmt.Errorf("mpv: %s", message.Error)
				}
				ch <- result
			}
			continue
		}
		c.dispatchEvent(message)
	}
	if err := scanner.Err(); err != nil {
		c.fail(err)
	}
}

func (c *ipc) dispatchEvent(message rpcMessage) {
	var event Event
	switch message.Event {
	case "file-loaded":
		event.Kind = EventFileLoaded
	case "end-file":
		event.Kind = EventEnded
		event.Reason = message.Reason
		if message.Reason == "error" {
			event.Err = errors.New("mpv could not play the stream")
		}
	case "idle", "start-file":
		return
	case "shutdown":
		return
	default:
		return
	}

	select {
	case c.events <- event:
	default:
		// A full UI channel must never stall mpv's reader. Polling state is the
		// source of truth; events are hints.
	}
}

func (c *ipc) fail(err error) {
	c.mu.Lock()
	c.readErr = err
	c.mu.Unlock()
	c.close()
}

func (c *ipc) connectionError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readErr != nil {
		return c.readErr
	}
	return io.ErrClosedPipe
}

func (c *ipc) Close() error {
	c.close()
	return nil
}

func (c *ipc) close() {
	c.closeOnce.Do(func() {
		_ = c.conn.Close()
		c.mu.Lock()
		for id, ch := range c.pending {
			ch <- rpcResult{err: io.ErrClosedPipe}
			delete(c.pending, id)
		}
		c.mu.Unlock()
	})
}
