// Package testcase reads test cases from txtar archives under testdata, see golang.org/x/tools/txtar.
//
// An archive holds one or more cases, and a case is a group of sections. A new case starts at
// a section whose name is already in the current case, so cases usually start with the same
// section, e.g.
//
//	Comments before the first section are ignored.
//	-- input --
//	example.com
//	-- output --
//	example.com
//	-- input --
//	example..com
//	-- error --
//	invalid domain
package testcase

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"
)

// Case is a test case, which holds the contents of its sections by name.
// Contents end with a newline unless they are empty.
type Case struct {
	Name     string // path and line of the first section, e.g. "testdata/text.txtar:12"
	Sections map[string]string
}

// Lookup returns the content of the section and whether it exists.
func (c Case) Lookup(name string) (string, bool) {
	content, ok := c.Sections[name]
	return content, ok
}

// Get returns the content of the section, it fails the test if the section does not exist.
func (c Case) Get(t testing.TB, name string) string {
	t.Helper()
	content, ok := c.Sections[name]
	if !ok {
		t.Fatalf("%s: missing section %q", c.Name, name)
	}
	return content
}

// Read reads the cases of the archives matching the pattern, e.g. "testdata/*.txtar",
// it fails the test if no archive matches or an archive has no case.
func Read(t testing.TB, pattern string) []Case {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no archive matches %q", pattern)
	}
	var cases []Case
	for _, path := range paths {
		archive, err := txtar.ParseFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(archive.Files) == 0 {
			t.Fatalf("%s: no case", path)
		}
		cases = append(cases, split(path, archive)...)
	}
	return cases
}

// Run runs each case of the archives matching the pattern as a subtest named by Case.Name.
func Run(t *testing.T, pattern string, f func(t *testing.T, c Case)) {
	t.Helper()
	for _, c := range Read(t, pattern) {
		t.Run(c.Name, func(t *testing.T) { f(t, c) })
	}
}

func split(path string, archive *txtar.Archive) []Case {
	var cases []Case
	// Parse ends the comment and contents with a newline, so the lines could be counted.
	line := 1 + bytes.Count(archive.Comment, []byte("\n"))
	for _, file := range archive.Files {
		if len(cases) == 0 || hasSection(cases[len(cases)-1], file.Name) {
			cases = append(cases, Case{Name: fmt.Sprintf("%s:%d", path, line), Sections: make(map[string]string)})
		}
		cases[len(cases)-1].Sections[file.Name] = string(file.Data)
		line += 1 + bytes.Count(file.Data, []byte("\n"))
	}
	return cases
}

func hasSection(c Case, name string) bool {
	_, ok := c.Sections[name]
	return ok
}
