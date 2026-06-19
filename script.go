package basremote

import (
	"encoding/json"

	"github.com/Masterminds/semver/v3"
)

var minSupportedVersion = semver.MustParse("22.4.2")

// Script holds metadata returned by the BAS script-properties API.
type Script struct {
	data map[string]interface{}
}

// NewScript parses the JSON body returned by the properties endpoint.
func NewScript(body []byte) (*Script, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &Script{data: data}, nil
}

// IsExist reports whether the script was found.
func (s *Script) IsExist() bool {
	v, _ := s.data["success"].(bool)
	return v
}

// IsFree reports whether the script is freely accessible.
func (s *Script) IsFree() bool {
	v, _ := s.data["free"].(bool)
	return v
}

// Hash returns the script content hash (first 5 chars used for exe dir naming).
func (s *Script) Hash() string {
	v, _ := s.data["hash"].(string)
	return v
}

// EngineVersion returns the engine version string required by this script.
func (s *Script) EngineVersion() string {
	v, _ := s.data["engversion"].(string)
	return v
}

// IsSupported reports whether the engine version meets the minimum requirement (22.4.2+).
func (s *Script) IsSupported() bool {
	ev := s.EngineVersion()
	if ev == "" {
		return false
	}
	parsed, err := semver.NewVersion(ev)
	if err != nil {
		return false
	}
	return parsed.Compare(minSupportedVersion) >= 0
}
