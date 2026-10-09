package singjson

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListableUnmarshal(t *testing.T) {
	var l Listable[int]
	require.NoError(t, UnmarshalStrict([]byte(`1`), &l))
	require.Equal(t, Listable[int]{1}, l)
	require.NoError(t, UnmarshalStrict([]byte(`[2,3]`), &l))
	require.Equal(t, Listable[int]{2, 3}, l)
	require.NoError(t, UnmarshalStrict([]byte(`null`), &l))
	require.Nil(t, l)
}
