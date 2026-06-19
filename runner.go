package basremote

import "encoding/json"

// runResult carries the outcome of a BAS task: either a value or an error.
type runResult struct {
	Value json.RawMessage
	Err   error
}

// basRunner is the shared base for BasFunction and BasThread.
type basRunner struct {
	id     int
	client *BasRemoteClient
	ch     chan runResult
}

func newRunner(c *BasRemoteClient) basRunner {
	return basRunner{
		client: c,
		ch:     make(chan runResult, 1),
	}
}

// Result returns the channel that delivers exactly one runResult when the task finishes.
func (r *basRunner) Result() <-chan runResult { return r.ch }

// runTask sends a "run_task" message and pushes the result into r.ch.
func (r *basRunner) runTask(name string, params map[string]interface{}) {
	p := params
	if p == nil {
		p = map[string]interface{}{}
	}
	paramsJSON, _ := json.Marshal(p)

	raw, err := r.client.sendAsync("run_task", map[string]interface{}{
		"params":        string(paramsJSON),
		"function_name": name,
		"thread_id":     r.id,
	})
	if err != nil {
		r.ch <- runResult{Err: err}
		return
	}

	resp, err := ParseResponse(string(raw))
	if err != nil {
		r.ch <- runResult{Err: err}
		return
	}
	if !resp.Success {
		r.ch <- runResult{Err: &FunctionError{Msg: resp.Message}}
		return
	}
	r.ch <- runResult{Value: resp.Result}
}
