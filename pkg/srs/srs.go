package srs

import (
	"encoding/json"
	"fmt"

	"github.com/imduaky/ruleset/pkg/internal/singjson"
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
	//TODO implement me
	panic("implement me")
}

func (r *Ruleset) UnmarshalSrsBinary(data []byte) error {
	//TODO implement me
	panic("implement me")
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

	// assume the Version is correctly
	var rr Ruleset
	err = singjson.UnmarshalStrict(data, &rr)
	if err != nil {
		return fmt.Errorf("unmarshalSrsJSON: %w", err)
	}
	return nil
}
