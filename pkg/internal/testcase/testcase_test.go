package testcase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

const archive = `comment
second comment line
-- input --
a
-- output --
b
c
-- input --
-- error --
oops
-- input --
d
-- extra --
`

func TestSplit(t *testing.T) {
	cases := split("x.txtar", txtar.Parse([]byte(archive)))
	require.Equal(t, []Case{
		{Name: "x.txtar:3", Sections: map[string]string{"input": "a\n", "output": "b\nc\n"}},
		{Name: "x.txtar:8", Sections: map[string]string{"input": "", "error": "oops\n"}},
		{Name: "x.txtar:11", Sections: map[string]string{"input": "d\n", "extra": ""}},
	}, cases)

	// No comment.
	cases = split("y", txtar.Parse([]byte("-- a --\n-- b --\n1\n-- b --\n2\n")))
	require.Equal(t, []string{"y:1", "y:4"}, []string{cases[0].Name, cases[1].Name})
	require.Equal(t, map[string]string{"b": "2\n"}, cases[1].Sections)
}

func TestRead(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txtar"), []byte(archive), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txtar"), []byte("-- input --\ne\n"), 0o644))

	cases := Read(t, filepath.Join(dir, "*.txtar"))
	require.Len(t, cases, 4)
	require.Equal(t, filepath.Join(dir, "b.txtar")+":1", cases[3].Name)

	content, ok := cases[0].Lookup("output")
	require.True(t, ok)
	require.Equal(t, "b\nc\n", content)
	require.Equal(t, "oops\n", cases[1].Get(t, "error"))
	_, ok = cases[1].Lookup("output")
	require.False(t, ok)

	var names []string
	Run(t, filepath.Join(dir, "b.txtar"), func(t *testing.T, c Case) {
		names = append(names, c.Name)
		require.Equal(t, "e\n", c.Get(t, "input"))
	})
	require.Equal(t, []string{filepath.Join(dir, "b.txtar") + ":1"}, names)
}
