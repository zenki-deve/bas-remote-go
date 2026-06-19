package basremote

import (
	"encoding/json"
	"fmt"
)

const messageSeparator = "---Message--End---"

// Message represents a single BAS WebSocket protocol message.
type Message struct {
	Async bool            `json:"async"`
	Type  string          `json:"type"`
	ID    int             `json:"id"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// ToJSON serialises the message to a JSON string.
func (m Message) ToJSON() string {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

// MessageFromJSON deserialises a JSON string into a Message.
func MessageFromJSON(s string) (Message, error) {
	var m Message
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return Message{}, err
	}
	return m, nil
}

// DataString returns the raw Data field as a plain string (stripping JSON quotes
// if it is a JSON string literal).
func (m Message) DataString() string {
	if len(m.Data) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(m.Data, &s); err == nil {
		return s
	}
	return string(m.Data)
}
