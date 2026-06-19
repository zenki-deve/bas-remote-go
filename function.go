package basremote

import (
	"math/rand"
)

// BasFunction represents a one-shot BAS function call.
// It starts a new thread, runs the function, then stops the thread.
type BasFunction struct {
	basRunner
}

func newBasFunction(c *BasRemoteClient, name string, params map[string]interface{}) *BasFunction {
	f := &BasFunction{basRunner: newRunner(c)}
	go f.run(name, params)
	return f
}

func (f *BasFunction) run(name string, params map[string]interface{}) {
	f.id = rand.Intn(1_000_000) + 1
	if err := f.client.startThread(f.id); err != nil {
		f.ch <- runResult{Err: err}
		return
	}
	f.runTask(name, params)
	// Best-effort stop; ignore error since result already delivered.
	_ = f.client.stopThread(f.id)
}

// Stop immediately stops the function's thread.
func (f *BasFunction) Stop() error {
	return f.client.stopThread(f.id)
}
