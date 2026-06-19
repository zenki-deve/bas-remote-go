package basremote

import (
	"math/rand"
	"sync/atomic"
)

// BasThread is a reusable BAS thread. Unlike BasFunction it does not
// automatically stop after the first function call.
type BasThread struct {
	basRunner
	isRunning atomic.Bool
}

func newBasThread(c *BasRemoteClient) *BasThread {
	return &BasThread{basRunner: newRunner(c)}
}

// RunFunction queues a BAS function on this thread and returns self for chaining.
// Returns ErrAlreadyRunning if a function is already executing on this thread.
func (t *BasThread) RunFunction(name string, params map[string]interface{}) (*BasThread, error) {
	if t.isRunning.Load() {
		return nil, ErrAlreadyRunning
	}
	// Reset result channel for this call.
	t.ch = make(chan runResult, 1)
	go t.run(name, params)
	return t, nil
}

func (t *BasThread) run(name string, params map[string]interface{}) {
	if t.id == 0 {
		t.id = rand.Intn(1_000_000) + 1
		if err := t.client.startThread(t.id); err != nil {
			t.ch <- runResult{Err: err}
			return
		}
	}
	t.isRunning.Store(true)
	t.runTask(name, params)
	t.isRunning.Store(false)
}

// Stop stops the thread and resets its state.
func (t *BasThread) Stop() error {
	if t.id == 0 {
		return nil
	}
	err := t.client.stopThread(t.id)
	t.isRunning.Store(false)
	t.id = 0
	return err
}

// IsRunning reports whether the thread is currently executing a function.
func (t *BasThread) IsRunning() bool { return t.isRunning.Load() }
