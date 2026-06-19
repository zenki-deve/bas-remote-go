package basremote

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// ----- Options -----

func TestOptionsValidation_MissingScriptName(t *testing.T) {
	opts := &Options{}
	if err := opts.validate(); err == nil {
		t.Fatal("expected error for empty ScriptName, got nil")
	}
}

func TestOptionsValidation_DefaultWorkingDir(t *testing.T) {
	opts := &Options{ScriptName: "test"}
	if err := opts.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WorkingDir == "" {
		t.Fatal("WorkingDir should be set to a default")
	}
	cwd, _ := os.Getwd()
	if !strings.HasPrefix(opts.WorkingDir, cwd[:3]) { // same drive letter at minimum
		t.Fatalf("WorkingDir %q does not look like an absolute path", opts.WorkingDir)
	}
}

func TestOptionsValidation_CustomWorkingDir(t *testing.T) {
	opts := &Options{ScriptName: "test", WorkingDir: "relative/path"}
	if err := opts.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(opts.WorkingDir, "relative") {
		t.Fatalf("expected WorkingDir to contain 'relative', got %q", opts.WorkingDir)
	}
	// Must be absolute after validate().
	if !strings.ContainsAny(opts.WorkingDir[:2], ":/\\") {
		t.Fatalf("expected absolute path, got %q", opts.WorkingDir)
	}
}

// ----- Message -----

func TestMessageRoundTrip(t *testing.T) {
	original := Message{
		Async: true,
		Type:  "run_task",
		ID:    123456,
		Data:  json.RawMessage(`{"key":"value"}`),
	}
	serialised := original.ToJSON()

	parsed, err := MessageFromJSON(serialised)
	if err != nil {
		t.Fatalf("MessageFromJSON error: %v", err)
	}
	if parsed.Async != original.Async {
		t.Errorf("Async: want %v got %v", original.Async, parsed.Async)
	}
	if parsed.Type != original.Type {
		t.Errorf("Type: want %q got %q", original.Type, parsed.Type)
	}
	if parsed.ID != original.ID {
		t.Errorf("ID: want %d got %d", original.ID, parsed.ID)
	}
	if string(parsed.Data) != string(original.Data) {
		t.Errorf("Data: want %s got %s", original.Data, parsed.Data)
	}
}

func TestMessageFromJSON_Invalid(t *testing.T) {
	_, err := MessageFromJSON("not-json{{{")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestMessageDataString_StringLiteral(t *testing.T) {
	m := Message{Data: json.RawMessage(`"hello world"`)}
	if got := m.DataString(); got != "hello world" {
		t.Errorf("DataString: want %q got %q", "hello world", got)
	}
}

func TestMessageDataString_Object(t *testing.T) {
	m := Message{Data: json.RawMessage(`{"a":1}`)}
	if got := m.DataString(); got != `{"a":1}` {
		t.Errorf("DataString: want %q got %q", `{"a":1}`, got)
	}
}

// ----- Message separator parsing (socketService.processData) -----

func TestProcessData_SingleMessage(t *testing.T) {
	received := []Message{}
	svc := &socketService{
		emit: func(event string, data interface{}) {
			if event == "message_received" {
				received = append(received, data.(Message))
			}
		},
	}
	payload := `{"async":false,"type":"initialize","id":1}` + messageSeparator
	svc.processData(payload)

	if len(received) != 1 {
		t.Fatalf("want 1 message, got %d", len(received))
	}
	if received[0].Type != "initialize" {
		t.Errorf("Type: want %q got %q", "initialize", received[0].Type)
	}
}

func TestProcessData_MultipleMessages(t *testing.T) {
	received := []Message{}
	svc := &socketService{
		emit: func(event string, data interface{}) {
			if event == "message_received" {
				received = append(received, data.(Message))
			}
		},
	}
	sep := messageSeparator
	payload := `{"async":false,"type":"A","id":1}` + sep +
		`{"async":false,"type":"B","id":2}` + sep
	svc.processData(payload)

	if len(received) != 2 {
		t.Fatalf("want 2 messages, got %d", len(received))
	}
	if received[0].Type != "A" || received[1].Type != "B" {
		t.Errorf("wrong types: %q %q", received[0].Type, received[1].Type)
	}
}

func TestProcessData_PartialMessage(t *testing.T) {
	received := []Message{}
	svc := &socketService{
		emit: func(event string, data interface{}) {
			if event == "message_received" {
				received = append(received, data.(Message))
			}
		},
	}
	// Send the message in two chunks split mid-separator.
	sep := messageSeparator
	full := `{"async":false,"type":"X","id":99}` + sep
	half := len(sep) / 2
	svc.processData(full[:len(full)-half])
	svc.processData(full[len(full)-half:])

	if len(received) != 1 {
		t.Fatalf("want 1 message after two chunks, got %d", len(received))
	}
	if received[0].Type != "X" {
		t.Errorf("Type: want %q got %q", "X", received[0].Type)
	}
}

// ----- Script version -----

func TestScriptVersionCheck_Supported(t *testing.T) {
	cases := []string{"22.4.2", "22.4.3", "23.0.0", "100.0.0"}
	for _, v := range cases {
		body := makeScriptBody(v, true)
		s, err := NewScript(body)
		if err != nil {
			t.Fatalf("NewScript error: %v", err)
		}
		if !s.IsSupported() {
			t.Errorf("version %q should be supported", v)
		}
	}
}

func TestScriptVersionCheck_NotSupported(t *testing.T) {
	cases := []string{"22.4.1", "22.3.9", "21.0.0", "1.0.0", ""}
	for _, v := range cases {
		body := makeScriptBody(v, true)
		s, err := NewScript(body)
		if err != nil {
			t.Fatalf("NewScript error: %v", err)
		}
		if s.IsSupported() {
			t.Errorf("version %q should NOT be supported", v)
		}
	}
}

func TestScriptIsExist(t *testing.T) {
	s, _ := NewScript(makeScriptBody("22.4.2", true))
	if !s.IsExist() {
		t.Error("IsExist should be true")
	}
	s2, _ := NewScript(makeScriptBody("22.4.2", false))
	if s2.IsExist() {
		t.Error("IsExist should be false")
	}
}

func makeScriptBody(version string, exists bool) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"engversion": version,
		"success":    exists,
		"free":       false,
		"hash":       "abcdef1234",
	})
	return b
}

// ----- Response -----

func TestResponseParsing_Success(t *testing.T) {
	raw := `{"Success":true,"Message":"","Result":"some_value"}`
	r, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if !r.Success {
		t.Error("Success should be true")
	}
}

func TestResponseParsing_Failure(t *testing.T) {
	raw := `{"Success":false,"Message":"something went wrong","Result":null}`
	r, err := ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse error: %v", err)
	}
	if r.Success {
		t.Error("Success should be false")
	}
	if r.Message != "something went wrong" {
		t.Errorf("Message: want %q got %q", "something went wrong", r.Message)
	}
}

// ----- Error types -----

func TestErrorTypes_SentinelMessages(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{ErrSocketNotConnected, "connect"},
		{ErrScriptNotSupported, "22.4.2"},
		{ErrClientNotStarted, "not started"},
		{ErrScriptNotExist, "not exist"},
		{ErrAuthentication, "authentication"},
		{ErrAlreadyRunning, "already running"},
	}
	for _, c := range cases {
		msg := c.err.Error()
		if !strings.Contains(strings.ToLower(msg), strings.ToLower(c.want)) {
			t.Errorf("error %v: want substring %q in message %q", c.err, c.want, msg)
		}
	}
}

func TestFunctionError_ImplementsError(t *testing.T) {
	var _ error = &FunctionError{}
	fe := &FunctionError{Msg: "custom BAS error"}
	if fe.Error() != "custom BAS error" {
		t.Errorf("FunctionError.Error(): got %q", fe.Error())
	}
}

func TestFunctionError_Unwrap(t *testing.T) {
	fe := &FunctionError{Msg: "boom"}
	// FunctionError is a leaf; errors.Is should match by value.
	if !errors.As(fe, new(*FunctionError)) {
		t.Error("errors.As(*FunctionError) should succeed")
	}
}
