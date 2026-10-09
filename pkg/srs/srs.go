package srs

import (
	"bytes"
	"encoding/json/v2"
	"fmt"

	"github.com/imduaky/ruleset/pkg/srs/singjson"
)

type BinarySrsMarshaler interface {
	MarshalSrsBinary() ([]byte, error)
}
type JSONSrsMarshaler interface {
	MarshalSrsJSON() ([]byte, error)
}

type BinarySrsUnmarshaler interface {
	UnmarshalSrsBinary(data []byte) error
}

type JSONSrsUnmarshaler interface {
	UnmarshalSrsJson(data []byte) error
}

type Srs interface {
	BinarySrsMarshaler
	BinarySrsUnmarshaler
	JSONSrsMarshaler
	JSONSrsUnmarshaler
}

var _ Srs = (*Ruleset)(nil)

type Ruleset struct {
	Version RuleSetVersion `json:"version"`
	Rules   []Rule         `json:"rules"`
}

type _RuleSet Ruleset

func (r Ruleset) MarshalSrsBinary() ([]byte, error) {
	var buffer bytes.Buffer
	err := Write(&buffer, r)
	if err != nil {
		return nil, fmt.Errorf("marshalSrsBinary: %w", err)
	}
	return buffer.Bytes(), nil
}

func (r *Ruleset) UnmarshalSrsBinary(data []byte) error {
	rr, err := Read(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("unmarshalSrsBinary: %w", err)
	}
	*r = *rr
	return nil
}

func (r Ruleset) MarshalSrsJSON() ([]byte, error) {
	data, err := json.Marshal((_RuleSet)(r))
	if err != nil {
		return nil, fmt.Errorf("marshalSrsJSON: %w", err)
	}
	return data, nil
}

func (r *Ruleset) UnmarshalSrsJson(data []byte) error {
	type versionedRuleset struct {
		Version RuleSetVersion `json:"version"`
	}
	var vv versionedRuleset
	err := json.Unmarshal(data, &vv)
	if err != nil {
		return fmt.Errorf("unmarshalSrsJSON: %w", err)
	}

	var rr Ruleset
	err = singjson.UnmarshalStrict(data, &rr)
	if err != nil {
		return fmt.Errorf("unmarshalSrsJSON: %w", err)
	}
	*r = rr
	return nil
}
