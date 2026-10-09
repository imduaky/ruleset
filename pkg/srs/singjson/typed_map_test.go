package singjson

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

type entries = []MapEntry[string, int]

func TestTypedMapKeepOrder(t *testing.T) {
	content := `{"c":1,"a":2,"b":3}`
	m := new(TypedMap[string, int])
	require.NoError(t, UnmarshalStrict([]byte(content), m))
	require.Equal(t, entries{{Key: "c", Value: 1}, {Key: "a", Value: 2}, {Key: "b", Value: 3}}, m.Entries())

	data, err := json.Marshal(m)
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func TestTypedMapPutKeepPosition(t *testing.T) {
	m := new(TypedMap[string, int])
	require.NoError(t, UnmarshalStrict([]byte(`{"a":1,"b":2}`), m))
	m.Put("a", 3)
	require.Equal(t, entries{{Key: "a", Value: 3}, {Key: "b", Value: 2}}, m.Entries())
}

func TestTypedMapUnmarshal(t *testing.T) {
	m := new(TypedMap[string, int])
	m.Put("a", 1)
	require.NoError(t, UnmarshalStrict([]byte(`null`), m))
	require.True(t, m.IsEmpty())

	require.Error(t, UnmarshalStrict([]byte(`{"a":1,"a":2}`), m))
	require.Error(t, UnmarshalStrict([]byte(`[1]`), m))
	require.Error(t, UnmarshalStrict([]byte(`{"a":"1"}`), m))
}

func TestTypedMapField(t *testing.T) {
	type object struct {
		Map *TypedMap[string, int] `json:"map,omitempty"`
	}
	for _, value := range []object{{}, {Map: new(TypedMap[string, int])}} {
		data, err := json.Marshal(value)
		require.NoError(t, err)
		require.Equal(t, `{}`, string(data))
	}

	var value object
	require.NoError(t, UnmarshalStrict([]byte(`{"map":{"b":1,"a":2}}`), &value))
	require.Equal(t, entries{{Key: "b", Value: 1}, {Key: "a", Value: 2}}, value.Map.Entries())
	data, err := json.Marshal(value)
	require.NoError(t, err)
	require.Equal(t, `{"map":{"b":1,"a":2}}`, string(data))
}
