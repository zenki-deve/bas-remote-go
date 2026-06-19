package basremote

import (
	"context"
	"encoding/json"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// BasRemoteClient provides methods for remotely controlling a BAS engine.
type BasRemoteClient struct {
	Options *Options

	engine    *engineService
	socket    *socketService
	isStarted atomic.Bool

	// startedCh receives nil on successful init or an error on auth failure.
	startedCh chan error

	// requests maps message IDs to response channels registered by sendAsync.
	requests sync.Map // map[int]chan json.RawMessage

	// handlers holds registered event callbacks.
	handlers map[string][]func(interface{})
	hMu      sync.RWMutex
}

// New creates a new BasRemoteClient. Call Start() to connect.
func New(opts *Options) (*BasRemoteClient, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	c := &BasRemoteClient{
		Options:   opts,
		startedCh: make(chan error, 1),
		handlers:  make(map[string][]func(interface{})),
	}
	c.engine = newEngineService(opts)
	c.socket = newSocketService(c.emit)

	c.on("message_received", func(v interface{}) {
		if msg, ok := v.(Message); ok {
			c.onMessageReceived(msg)
		}
	})
	c.on("socket_open", func(_ interface{}) {
		c.onSocketOpen()
	})
	return c, nil
}

// Start initialises the engine, connects the WebSocket and waits for the
// BAS thread_start handshake. timeout ≤ 0 means no timeout.
func (c *BasRemoteClient) Start(timeout time.Duration) error {
	if err := c.engine.initialize(); err != nil {
		return err
	}
	port := rand.Intn(10001) + 10000 // [10000, 20000]
	if err := c.engine.start(port); err != nil {
		return err
	}
	if err := c.socket.start(port); err != nil {
		return err
	}

	if timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		select {
		case err := <-c.startedCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return <-c.startedCh
}

// IsStarted reports whether the client has completed initialisation.
func (c *BasRemoteClient) IsStarted() bool { return c.isStarted.Load() }

// ----- internal event bus -----

func (c *BasRemoteClient) on(event string, fn func(interface{})) {
	c.hMu.Lock()
	c.handlers[event] = append(c.handlers[event], fn)
	c.hMu.Unlock()
}

func (c *BasRemoteClient) emit(event string, data interface{}) {
	c.hMu.RLock()
	fns := c.handlers[event]
	c.hMu.RUnlock()
	for _, fn := range fns {
		fn(data)
	}
}

// ----- protocol handlers -----

func (c *BasRemoteClient) onSocketOpen() {
	_, _ = c.send("remote_control_data", map[string]interface{}{
		"script":   c.Options.ScriptName,
		"password": c.Options.Password,
		"login":    c.Options.Login,
	}, false)
}

func (c *BasRemoteClient) onMessageReceived(msg Message) {
	switch msg.Type {
	case "initialize":
		_, _ = c.send("accept_resources", map[string]interface{}{"-bas-empty-script-": true}, false)

	case "thread_start":
		if !c.isStarted.Swap(true) {
			select {
			case c.startedCh <- nil:
			default:
			}
		}

	case "message":
		if !c.isStarted.Load() {
			select {
			case c.startedCh <- ErrAuthentication:
			default:
			}
		}

	default:
		if msg.Async && msg.ID != 0 {
			if ch, ok := c.requests.LoadAndDelete(msg.ID); ok {
				respCh := ch.(chan json.RawMessage)
				respCh <- msg.Data
			}
		}
	}
}

// ----- public API -----

// Send sends a message and returns its ID.
func (c *BasRemoteClient) Send(type_ string, data map[string]interface{}, async_ bool) (int, error) {
	if !c.IsStarted() {
		return 0, ErrClientNotStarted
	}
	return c.send(type_, data, async_)
}

// SendAsync sends a message and blocks until the response arrives.
func (c *BasRemoteClient) SendAsync(type_ string, data map[string]interface{}) (json.RawMessage, error) {
	if !c.IsStarted() {
		return nil, ErrClientNotStarted
	}
	return c.sendAsync(type_, data)
}

func (c *BasRemoteClient) send(type_ string, data map[string]interface{}, async_ bool) (int, error) {
	id := rand.Intn(900000) + 100000 // [100000, 999999]
	var rawData json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return 0, err
		}
		rawData = b
	} else {
		rawData = json.RawMessage("{}")
	}
	msg := Message{
		Async: async_,
		Type:  type_,
		ID:    id,
		Data:  rawData,
	}
	return c.socket.send(msg)
}

func (c *BasRemoteClient) sendAsync(type_ string, data map[string]interface{}) (json.RawMessage, error) {
	ch := make(chan json.RawMessage, 1)
	id := rand.Intn(900000) + 100000

	var rawData json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		rawData = b
	} else {
		rawData = json.RawMessage("{}")
	}

	c.requests.Store(id, ch)
	msg := Message{Async: true, Type: type_, ID: id, Data: rawData}
	if _, err := c.socket.send(msg); err != nil {
		c.requests.Delete(id)
		return nil, err
	}
	return <-ch, nil
}

// StartThread sends a start_thread command for the given thread ID.
func (c *BasRemoteClient) StartThread(threadID int) error {
	_, err := c.Send("start_thread", map[string]interface{}{"thread_id": threadID}, false)
	return err
}

func (c *BasRemoteClient) startThread(threadID int) error {
	_, err := c.send("start_thread", map[string]interface{}{"thread_id": threadID}, false)
	return err
}

// StopThread sends a stop_thread command for the given thread ID.
func (c *BasRemoteClient) StopThread(threadID int) error {
	_, err := c.Send("stop_thread", map[string]interface{}{"thread_id": threadID}, false)
	return err
}

func (c *BasRemoteClient) stopThread(threadID int) error {
	_, err := c.send("stop_thread", map[string]interface{}{"thread_id": threadID}, false)
	return err
}

// RunFunction starts a one-shot BAS function call in a new thread.
func (c *BasRemoteClient) RunFunction(name string, params map[string]interface{}) (*BasFunction, error) {
	if !c.IsStarted() {
		return nil, ErrClientNotStarted
	}
	return newBasFunction(c, name, params), nil
}

// CreateThread creates a new reusable BAS thread object.
func (c *BasRemoteClient) CreateThread() *BasThread {
	return newBasThread(c)
}

// Close shuts down the WebSocket connection and kills the engine process.
func (c *BasRemoteClient) Close() error {
	_ = c.socket.close()
	_ = c.engine.close()
	c.isStarted.Store(false)
	return nil
}
