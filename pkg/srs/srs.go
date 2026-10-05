package srs

import (
	"context"
)

type BinarySrsMarshaler interface {
	MarshalSrsBinary(ctx context.Context) ([]byte, error)
}
type JSONSrsMarshaler interface {
	MarshalSrsJSON(ctx context.Context) ([]byte, error)
}

type BinarySrsUnmarshaler interface {
	UnmarshalSrsBinary(ctx context.Context, data []byte) error
}

type JSONSrsUnmarshaler interface {
	UnmarshalSrsJson(ctx context.Context, data []byte) error
}

type Srs interface {
	BinarySrsMarshaler
	BinarySrsUnmarshaler
	JSONSrsMarshaler
	JSONSrsUnmarshaler
}

var _ Srs = (*Ruleset)(nil)

type Ruleset struct {
}

func (r Ruleset) MarshalSrsBinary(ctx context.Context) ([]byte, error) {

}

func (r *Ruleset) UnmarshalSrsBinary(ctx context.Context, data []byte) error {
	//TODO implement me
	panic("implement me")
}

func (r Ruleset) MarshalSrsJSON(ctx context.Context) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *Ruleset) UnmarshalSrsJson(ctx context.Context, data []byte) error {
	//TODO implement me
	panic("implement me")
}
