package basremote

import "errors"

var (
	ErrSocketNotConnected = errors.New("cannot connect to the WebSocket server")
	ErrScriptNotSupported = errors.New("script engine not supported (required 22.4.2 or newer)")
	ErrClientNotStarted   = errors.New("request can not be sent: client is not started")
	ErrScriptNotExist     = errors.New("script with selected name does not exist")
	ErrAuthentication     = errors.New("unsuccessful authentication")
	ErrAlreadyRunning     = errors.New("another task is already running, unable to start a new one")
)

// FunctionError is returned when BAS reports a function-level failure.
type FunctionError struct {
	Msg string
}

func (e *FunctionError) Error() string { return e.Msg }
