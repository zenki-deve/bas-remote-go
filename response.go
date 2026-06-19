package basremote

import "encoding/json"

// Response is the payload returned by BAS for async task requests.
type Response struct {
	Success bool            `json:"Success"`
	Message string          `json:"Message"`
	Result  json.RawMessage `json:"Result"`
}

// ParseResponse deserialises a JSON string into a Response.
func ParseResponse(s string) (Response, error) {
	var r Response
	// Try parsing direct JSON object first
	if err := json.Unmarshal([]byte(s), &r); err == nil {
		return r, nil
	}

	// If that fails, it might be a JSON string literal containing the JSON object
	var unescaped string
	if err := json.Unmarshal([]byte(s), &unescaped); err == nil {
		if err := json.Unmarshal([]byte(unescaped), &r); err == nil {
			return r, nil
		}
	}

	// Fallback to the original unmarshal error
	var errResponse Response
	err := json.Unmarshal([]byte(s), &errResponse)
	return Response{}, err
}
