package domainlist_test

import (
	"errors"
	"io/fs"
	"iter"
	"maps"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/imduaky/ruleset/pkg/domainlist"

	"github.com/stretchr/testify/require"
)

// get looks up the list of the name, it fails on an error.
func get(t *testing.T, l domainlist.Lookuper, name string) domainlist.DomainList {
	t.Helper()
	list, err := l.Lookup(name)
	require.NoError(t, err)
	return list
}

// setOf returns a set of the lists parsed from the texts, which are added in the order of names.
func setOf(t *testing.T, texts map[string]string) *domainlist.DomainListSet {
	set := domainlist.NewDomainListSet()
	for _, name := range slices.Sorted(maps.Keys(texts)) {
		set.Add(domainlist.NamedDomainList{Name: name, DomainList: parseText(t, texts[name])})
	}
	return set
}

// names collects the names of the lists, it fails on an error.
func names(t *testing.T, lists iter.Seq2[domainlist.NamedDomainList, error]) []string {
	t.Helper()
	var names []string
	for list, err := range lists {
		require.NoError(t, err)
		names = append(names, list.Name)
	}
	return names
}

func TestFileLookuper(t *testing.T) {
	opened := make(map[string]int)
	l := domainlist.FileLookuper{FS: countFS{FS: fstest.MapFS{
		"a":         {Data: []byte("include:b\na.com")},
		"b":         {Data: []byte("b.com @cn")},
		"unused":    {Data: []byte("unused.com")},
		"include":   {Data: []byte("include:bad")},
		"dir/c":     {Data: []byte("c.com")},
		".DS_Store": {Data: []byte("\x00")},
		"README.md": {Data: []byte("# Lists")},
	}, opened: opened}}
	bad := domainlist.FileLookuper{FS: fstest.MapFS{"a": {Data: []byte("a.com")}, "bad": {Data: []byte("bad.com attr")}}}

	list, err := l.Lookup("b")
	require.NoError(t, err)
	require.Equal(t, parseText(t, "b.com @cn"), list)
	for _, name := range []string{"missing", "bad", "dir", "dir/c", "../a", "A", "README.md", ""} {
		_, err = l.Lookup(name)
		require.ErrorIs(t, err, domainlist.ErrListNotFound, name)
	}
	_, err = bad.Lookup("bad")
	require.EqualError(t, err, `read bad: line 1: unknown field: "attr"`)

	// The directories and the files which are not lists are skipped.
	require.Equal(t, []string{"a", "b", "include", "unused"}, names(t, l.All()))
	for list, err := range l.All() {
		require.NoError(t, err)
		require.Equal(t, "a", list.Name)
		require.Equal(t, parseText(t, "include:b\na.com"), list.DomainList)
		break
	}
	var errs []error
	for _, err := range bad.All() {
		errs = append(errs, err)
	}
	require.Len(t, errs, 2)
	require.NoError(t, errs[0])
	require.EqualError(t, errs[1], `read bad: line 1: unknown field: "attr"`)

	// The directory is read in batches.
	many := make(fstest.MapFS)
	for i := range 200 {
		many["l"+strconv.Itoa(i)] = &fstest.MapFile{Data: []byte("l" + strconv.Itoa(i) + ".com")}
	}
	require.Len(t, names(t, domainlist.FileLookuper{FS: many}.All()), 200)

	for _, fsys := range []fs.FS{errFS{}, fileFS{}} {
		for _, err := range (domainlist.FileLookuper{FS: fsys}).All() {
			require.Error(t, err)
		}
	}

	// The files are read lazily, each once by a Merger.
	clear(opened)
	m := domainlist.NewMerger(l)
	for range 2 {
		merged, err := m.Merge("a")
		require.NoError(t, err)
		require.Equal(t, "a.com\nb.com @cn\n", marshal(t, merged))
	}
	require.Equal(t, map[string]int{"a": 1, "b": 1}, opened)
	_, err = m.Merge("include")
	require.EqualError(t, err, "list not found: include -> bad")

	// MergeAll looks up all lists before merging them, so that each file is read once.
	clear(opened)
	l.FS.(countFS).FS.(fstest.MapFS)["include"].Data = []byte("include:a")
	all, err := domainlist.NewMerger(l).MergeAll()
	require.NoError(t, err)
	require.Equal(t, 4, all.Len())
	require.Equal(t, map[string]int{".": 1, "a": 1, "b": 1, "include": 1, "unused": 1}, opened)
}

// countFS counts the opened files.
type countFS struct {
	fs.FS
	opened map[string]int
}

func (f countFS) Open(name string) (fs.File, error) {
	f.opened[name]++
	return f.FS.Open(name)
}

type errFS struct{}

func (errFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }

// fileFS opens a file which is not a directory.
type fileFS struct{}

func (fileFS) Open(string) (fs.File, error) { return fstest.MapFS{"f": {}}.Open("f") }

// countLookuper counts the lookups of a set.
type countLookuper struct {
	set *domainlist.DomainListSet
	err error // the error of All

	mu      sync.Mutex
	lookups map[string]int
	alls    int
}

func (l *countLookuper) Lookup(name string) (domainlist.DomainList, error) {
	l.mu.Lock()
	l.lookups[name]++
	l.mu.Unlock()
	return l.set.Lookup(name)
}

func (l *countLookuper) All() iter.Seq2[domainlist.NamedDomainList, error] {
	return func(yield func(domainlist.NamedDomainList, error) bool) {
		l.mu.Lock()
		l.alls++
		l.mu.Unlock()
		for list, err := range l.set.All() {
			if !yield(list, err) {
				return
			}
		}
		if l.err != nil {
			yield(domainlist.NamedDomainList{}, l.err)
		}
	}
}

func TestCachedLookuper(t *testing.T) {
	set := setOf(t, map[string]string{"a": "include:c\na.com", "b": "include:c @cn\nb.com", "c": "c.com\nc.cn @cn"})
	upstream := &countLookuper{set: set, lookups: make(map[string]int)}
	c := domainlist.NewCachedLookuper(upstream)

	for range 2 {
		list, err := c.Lookup("a")
		require.NoError(t, err)
		require.Equal(t, get(t, set, "a"), list)
		// Errors are not cached.
		_, err = c.Lookup("missing")
		require.ErrorIs(t, err, domainlist.ErrListNotFound)
	}
	require.Equal(t, map[string]int{"a": 1, "missing": 2}, upstream.lookups)

	// A stopped All does not complete the cache.
	for range c.All() {
		break
	}
	require.Equal(t, 1, upstream.alls)
	// Once All iterates all lists, they are iterated from the cache.
	for range 2 {
		require.Equal(t, []string{"a", "b", "c"}, names(t, c.All()))
	}
	require.Equal(t, 2, upstream.alls)
	_, err := c.Lookup("c")
	require.NoError(t, err)
	require.Equal(t, map[string]int{"a": 1, "missing": 2}, upstream.lookups)

	// Mergers share the cache.
	clear(upstream.lookups)
	c = domainlist.NewCachedLookuper(upstream)
	for _, name := range []string{"a", "b"} {
		_, err := domainlist.NewMerger(c).Merge(name)
		require.NoError(t, err)
	}
	require.Equal(t, map[string]int{"a": 1, "b": 1, "c": 1}, upstream.lookups)

	// An error of All is yielded, and the cache is not complete.
	upstream.err = errors.New("all error")
	c = domainlist.NewCachedLookuper(upstream)
	for range 2 {
		var err error
		for _, err = range c.All() {
		}
		require.ErrorIs(t, err, upstream.err)
	}
	_, err = domainlist.NewMerger(c).MergeAll()
	require.ErrorIs(t, err, upstream.err)
}

// TestCachedLookuperConcurrent shares a CachedLookuper by a LockedLookuper, which is run with -race.
func TestCachedLookuperConcurrent(t *testing.T) {
	data := make(fstest.MapFS)
	for i := range 50 {
		data["l"+strconv.Itoa(i)] = &fstest.MapFile{Data: []byte("include:l" + strconv.Itoa((i+1)%50+50*(i/49)) + "\nl" + strconv.Itoa(i) + ".com")}
	}
	data["l49"] = &fstest.MapFile{Data: []byte("l49.com")}
	c := domainlist.NewLockedLookuper(domainlist.NewCachedLookuper(domainlist.FileLookuper{FS: data}))

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			m := domainlist.NewMerger(c)
			switch i % 3 {
			case 0:
				all, err := m.MergeAll()
				require.NoError(t, err)
				require.Len(t, get(t, all, "l0").Domains, 50)
			case 1:
				list, err := m.Merge("l" + strconv.Itoa(i))
				require.NoError(t, err)
				require.Len(t, list.Domains, 50-i)
			default:
				for list, err := range c.All() {
					require.NoError(t, err)
					_, err = c.Lookup(list.Name)
					require.NoError(t, err)
				}
			}
		})
	}
	wg.Wait()
}

// unsafeLookuper is not safe for concurrent use.
type unsafeLookuper struct {
	set     *domainlist.DomainListSet
	calls   int
	running int // the running iterators
}

func (l *unsafeLookuper) Lookup(name string) (domainlist.DomainList, error) {
	l.calls++
	return l.set.Lookup(name)
}

func (l *unsafeLookuper) All() iter.Seq2[domainlist.NamedDomainList, error] {
	return func(yield func(domainlist.NamedDomainList, error) bool) {
		l.running++
		defer func() { l.running-- }()
		for list, err := range l.set.All() {
			l.calls++
			if !yield(list, err) {
				return
			}
		}
	}
}

// TestLockedLookuper is run with -race.
func TestLockedLookuper(t *testing.T) {
	set := setOf(t, map[string]string{"a": "include:b\na.com", "b": "b.com", "c": "c.com"})
	unsafe := &unsafeLookuper{set: set}
	l := domainlist.NewLockedLookuper(unsafe)

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			switch i % 3 {
			case 0:
				list, err := l.Lookup("a")
				require.NoError(t, err)
				require.Equal(t, get(t, set, "a"), list)
			case 1:
				// The lists could be looked up in the loop without a deadlock.
				for list, err := range l.All() {
					require.NoError(t, err)
					_, err = l.Lookup(list.Name)
					require.NoError(t, err)
				}
			default:
				for range l.All() {
					break
				}
			}
		})
	}
	wg.Wait()
	// The stopped iterators are finished.
	require.Zero(t, unsafe.running)
	require.Equal(t, []string{"a", "b", "c"}, names(t, l.All()))

	merged, err := domainlist.NewMerger(l).Merge("a")
	require.NoError(t, err)
	require.Equal(t, "a.com\nb.com\n", marshal(t, merged))
}
