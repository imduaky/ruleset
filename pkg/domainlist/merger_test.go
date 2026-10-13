package domainlist_test

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/imduaky/ruleset/pkg/domainlist"
	"github.com/imduaky/ruleset/pkg/internal/testcase"

	"github.com/stretchr/testify/require"
)

// namedLists returns the data/<name> sections of the case as lists sorted by name.
func namedLists(t *testing.T, c testcase.Case) []domainlist.NamedDomainList {
	var lists []domainlist.NamedDomainList
	for section, content := range c.Sections {
		if name, ok := strings.CutPrefix(section, "data/"); ok {
			lists = append(lists, domainlist.NamedDomainList{Name: name, DomainList: parseText(t, content)})
		}
	}
	slices.SortFunc(lists, func(a, b domainlist.NamedDomainList) int { return strings.Compare(a.Name, b.Name) })
	return lists
}

func marshal(t *testing.T, list domainlist.DomainList) string {
	t.Helper()
	data, err := list.MarshalText()
	require.NoError(t, err)
	return string(data)
}

// TestMerge runs the cases in testdata/merge, see testdata/merge/merge.txtar.
func TestMerge(t *testing.T) {
	testcase.Run(t, "testdata/merge/*.txtar", func(t *testing.T, c testcase.Case) {
		name := strings.TrimSpace(c.Get(t, "merge"))
		lists := namedLists(t, c)
		m := domainlist.NewMerger(domainlist.NewDomainListSet(lists...))

		if attrs, ok := c.Lookup("attrs"); ok {
			list, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).MergeAttrs(name, strings.Fields(attrs)...)
			require.NoError(t, err)
			require.Equal(t, c.Get(t, "filtered"), marshal(t, list), "attrs")
		}
		if once, ok := c.Lookup("once"); ok {
			// MergeOnce merges the original lists before Merge.
			list, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).MergeOnce(name)
			require.NoError(t, err)
			require.Equal(t, once, marshal(t, list), "once")
		}

		merged, err := m.Merge(name)
		// The given lists are not modified.
		require.Equal(t, namedLists(t, c), lists)
		if message, ok := c.Lookup("error"); ok {
			require.EqualError(t, err, strings.TrimSpace(message))
			require.Zero(t, merged)
			// The same error again.
			_, err = m.Merge(name)
			require.EqualError(t, err, strings.TrimSpace(message))
			return
		}
		require.NoError(t, err)
		require.Empty(t, merged.Inclusions)
		require.Equal(t, c.Get(t, "output"), marshal(t, merged))

		// The merged list is reused, and modifying a returned list does not affect the Merger.
		merged.Domains[0].Value = "modified"
		again, err := m.Merge(name)
		require.NoError(t, err)
		require.Equal(t, c.Get(t, "output"), marshal(t, again))

		// MergeAll merges the same, and all lists are merged.
		all, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).MergeAll()
		require.NoError(t, err)
		require.Equal(t, len(lists), all.Len())
		require.Equal(t, again, get(t, all, name))
		for list, err := range all.All() {
			require.NoError(t, err)
			name := list.Name
			require.Empty(t, list.Inclusions, name)
		}

		// MergeAttrs filters while merging, the same as filtering the merged list.
		for _, attrs := range attrCases(lists) {
			filtered, err := m.MergeAttrs(name, attrs...)
			require.NoError(t, err)
			must, ban := splitAttrs(attrs)
			require.Equal(t, again.Filter(must, ban), filtered.Domains, "%v", attrs)
		}
	})
}

// TestMergeAll runs the cases in testdata/mergeall, see testdata/mergeall/merged.txtar.
func TestMergeAll(t *testing.T) {
	testcase.Run(t, "testdata/mergeall/*.txtar", func(t *testing.T, c testcase.Case) {
		lists := namedLists(t, c)
		all, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).MergeAll()
		require.Equal(t, namedLists(t, c), lists)
		if message, ok := c.Lookup("error"); ok {
			require.EqualError(t, err, strings.TrimSpace(message))
			require.Nil(t, all)
			return
		}
		require.NoError(t, err)
		require.Equal(t, len(lists), all.Len())
		for section, content := range c.Sections {
			if name, ok := strings.CutPrefix(section, "all/"); ok {
				require.Equal(t, content, marshal(t, get(t, all, name)), name)
			}
		}
	})
}

func TestMerger(t *testing.T) {
	list := func(name, text string) domainlist.NamedDomainList {
		return domainlist.NamedDomainList{Name: name, DomainList: parseText(t, text)}
	}
	set := domainlist.NewDomainListSet(list("a", "include:b\na.com"), list("b", "b.com"), list("a", "include:c @cn\na.org"), list("c", "c.cn @cn\nc.com"))
	m := domainlist.NewMerger(set)

	// The lists of the same name are concatenated.
	merged, err := m.Merge("a")
	require.NoError(t, err)
	require.Equal(t, "a.com\na.org\nb.com\nc.cn @cn\n", marshal(t, merged))
	merged, err = m.MergeAttrs("a", "cn")
	require.NoError(t, err)
	require.Equal(t, "c.cn @cn\n", marshal(t, merged))

	for _, merge := range []func(string) (domainlist.DomainList, error){m.Merge, m.MergeOnce, func(name string) (domainlist.DomainList, error) { return m.MergeAttrs(name) }} {
		_, err = merge("missing")
		require.ErrorIs(t, err, domainlist.ErrListNotFound)
		require.EqualError(t, err, "list not found: missing")
	}
	m = domainlist.NewMerger(domainlist.NewDomainListSet(list("a", "include:missing"), list("b", "include:b")))
	_, err = m.MergeOnce("a")
	require.EqualError(t, err, "list not found: a -> missing")
	_, err = m.Merge("b")
	require.ErrorIs(t, err, domainlist.ErrCircularInclusion)

	all, err := domainlist.NewMerger(domainlist.NewDomainListSet()).MergeAll()
	require.NoError(t, err)
	require.Zero(t, all.Len())
}

// attrCases returns the attributes of MergeAttrs for the attributes in the lists, e.g. [a], [-a], [a -b].
func attrCases(lists []domainlist.NamedDomainList) [][]string {
	var attrs []string
	for _, list := range lists {
		for _, d := range list.Domains {
			attrs = append(attrs, d.Attrs...)
		}
	}
	slices.Sort(attrs)
	attrs = slices.Compact(attrs)
	cases := [][]string{nil}
	for i, attr := range attrs {
		cases = append(cases, []string{attr}, []string{"-" + attr})
		if i > 0 {
			cases = append(cases, []string{attrs[i-1], "-" + attr})
		}
	}
	return cases
}

func splitAttrs(attrs []string) (must, ban []string) {
	for _, attr := range attrs {
		if b, ok := strings.CutPrefix(attr, "-"); ok {
			ban = append(ban, b)
		} else {
			must = append(must, attr)
		}
	}
	return must, ban
}

// TestMergerKeepsNoState checks that the changes of the Lookuper are seen by the next calls.
func TestMergerKeepsNoState(t *testing.T) {
	set := setOf(t, map[string]string{"a": "include:c\na.com", "c": "c.com"})
	m := domainlist.NewMerger(set)
	merged, err := m.Merge("a")
	require.NoError(t, err)
	require.Equal(t, "a.com\nc.com\n", marshal(t, merged))

	set.Add(domainlist.NamedDomainList{Name: "c", DomainList: parseText(t, "include:d\nc.org")})
	set.Add(domainlist.NamedDomainList{Name: "d", DomainList: parseText(t, "d.com @cn")})
	merged, err = m.Merge("a")
	require.NoError(t, err)
	require.Equal(t, "a.com\nc.com\nc.org\nd.com @cn\n", marshal(t, merged))
	merged, err = m.MergeAttrs("a", "cn")
	require.NoError(t, err)
	require.Equal(t, "d.com @cn\n", marshal(t, merged))
	all, err := m.MergeAll()
	require.NoError(t, err)
	require.Equal(t, merged.Domains, get(t, all, "d").Domains)

	// A circle made later is found.
	set.Add(domainlist.NamedDomainList{Name: "d", DomainList: parseText(t, "include:a")})
	_, err = m.Merge("a")
	require.EqualError(t, err, "circular inclusion: [a -> c -> d -> a]")
}

func TestDomainListSet(t *testing.T) {
	set := domainlist.NewDomainListSet(
		domainlist.NamedDomainList{Name: "b", DomainList: parseText(t, "include:x\nb.com")},
		domainlist.NamedDomainList{Name: "a", DomainList: parseText(t, "a.com @ads")},
	)
	set.Add(domainlist.NamedDomainList{Name: "b", DomainList: parseText(t, "include:y @z\nb.org")})
	// The lists are kept in the order of being added, and the lists of the same name are concatenated.
	require.Equal(t, 2, set.Len())
	require.Equal(t, []string{"b", "a"}, names(t, set.All()))
	require.Equal(t, parseText(t, "include:x\ninclude:y @z\nb.com\nb.org"), get(t, set, "b"))
	for list, err := range set.All() {
		require.NoError(t, err)
		require.Equal(t, get(t, set, list.Name), list.DomainList)
		break // It stops.
	}
	_, err := set.Lookup("c")
	require.ErrorIs(t, err, domainlist.ErrListNotFound)

	// The added lists are copied.
	added := parseText(t, "c.com")
	set.Add(domainlist.NamedDomainList{Name: "c", DomainList: added})
	added.Domains[0].Value = "modified"
	require.Equal(t, parseText(t, "c.com"), get(t, set, "c"))

	// Copy is deep.
	copied := set.Copy()
	require.Equal(t, set, copied)
	get(t, copied, "a").Domains[0].Attrs[0] = "modified"
	get(t, copied, "b").Inclusions[1].MustAttrs[0] = "modified"
	get(t, copied, "b").Domains[0].Value = "modified"
	copied.Add(domainlist.NamedDomainList{Name: "d"})
	require.Equal(t, parseText(t, "a.com @ads"), get(t, set, "a"))
	require.Equal(t, parseText(t, "include:x\ninclude:y @z\nb.com\nb.org"), get(t, set, "b"))
	require.Equal(t, 3, set.Len())
	require.Nil(t, (*domainlist.DomainListSet)(nil).Copy())
}

// TestMergeDeep checks that deep inclusions are merged without recursion.
func TestMergeDeep(t *testing.T) {
	const depth = 100000
	lists := make([]domainlist.NamedDomainList, depth+1)
	for i := range depth {
		lists[i] = domainlist.NamedDomainList{Name: "l" + strconv.Itoa(i), DomainList: domainlist.DomainList{Inclusions: []domainlist.Inclusion{{Name: "l" + strconv.Itoa(i+1)}}}}
	}
	lists[depth] = domainlist.NamedDomainList{Name: "l" + strconv.Itoa(depth), DomainList: parseText(t, "example.com")}

	merged, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).Merge("l0")
	require.NoError(t, err)
	require.Equal(t, parseText(t, "example.com"), merged)

	// A circle at the bottom.
	lists[depth].DomainList = parseText(t, "include:l99998")
	_, err = domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).Merge("l0")
	// The message shortens the path, which is in the MergeError.
	require.EqualError(t, err, "circular inclusion: l0 -> l1 -> l2 -> ...(99995 lists)... -> [l99998 -> l99999 -> l100000 -> l99998]")
	var mergeErr *domainlist.MergeError
	require.ErrorAs(t, err, &mergeErr)
	require.Len(t, mergeErr.Path, depth+2)
	require.Equal(t, 99998, mergeErr.Cycle)
	require.Equal(t, "l99998", mergeErr.Path[len(mergeErr.Path)-1])
}

// TestFilter runs the cases in testdata/filter.txtar.
func TestFilter(t *testing.T) {
	testcase.Run(t, "testdata/filter.txtar", func(t *testing.T, c testcase.Case) {
		var inc domainlist.Inclusion
		require.NoError(t, inc.UnmarshalText([]byte("include:filter "+c.Get(t, "filter"))))
		list := parseText(t, c.Get(t, "input"))
		filtered := domainlist.DomainList{Domains: list.Filter(inc.MustAttrs, inc.BanAttrs)}
		require.Equal(t, c.Get(t, "output"), marshal(t, filtered))
	})
	require.Empty(t, domainlist.DomainList{}.Filter(nil, nil))
}

func BenchmarkMerge(b *testing.B) {
	// Each list includes the next 2 lists, so that most are reached in different ways.
	const n = 64
	lists := make([]domainlist.NamedDomainList, n)
	for i := range n {
		list := benchmarkList(500)
		list.Inclusions = nil
		for j := i + 1; j < min(i+3, n); j++ {
			list.Inclusions = append(list.Inclusions, domainlist.Inclusion{Name: "list" + strconv.Itoa(j)})
		}
		for k := range list.Domains {
			list.Domains[k].Value = strconv.Itoa(i) + list.Domains[k].Value
		}
		lists[i] = domainlist.NamedDomainList{Name: "list" + strconv.Itoa(i), DomainList: list}
	}
	b.ReportAllocs()
	for b.Loop() {
		list, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).Merge("list0")
		if err != nil {
			b.Fatal(err)
		}
		if len(list.Domains) != n*500 {
			b.Fatal(len(list.Domains))
		}
	}
}

// BenchmarkMergeAllDataFiles merges all lists in the data directory of v2fly/domain-list-community, e.g.
// DLC_DATA=domain-list-community/data go test -run xxx -bench MergeAllDataFiles ./pkg/domainlist
func BenchmarkMergeAllDataFiles(b *testing.B) {
	dir := os.Getenv("DLC_DATA")
	if dir == "" {
		b.Skip("DLC_DATA is not set")
	}
	var lists []domainlist.NamedDomainList
	for list, err := range (&domainlist.FileLookuper{FS: os.DirFS(dir)}).All() {
		require.NoError(b, err)
		lists = append(lists, list)
	}

	b.Run("set", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).MergeAll()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("file", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, err := domainlist.NewMerger(&domainlist.FileLookuper{FS: os.DirFS(dir)}).MergeAll()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestMergeError(t *testing.T) {
	names := func(n int) []string {
		s := make([]string, n)
		for i := range s {
			s[i] = "l" + strconv.Itoa(i)
		}
		return s
	}
	lookupErr := errors.New("read error")
	for message, err := range map[string]*domainlist.MergeError{
		"list not found: missing":                       {Err: domainlist.ErrListNotFound, Path: []string{"missing"}, Cycle: -1},
		"list not found: top -> a -> missing":           {Err: fmt.Errorf("%w: %q", domainlist.ErrListNotFound, "missing"), Path: []string{"top", "a", "missing"}, Cycle: -1},
		"lookup top -> bad: read error":                 {Err: lookupErr, Path: []string{"top", "bad"}, Cycle: -1},
		"circular inclusion: [a -> a]":                  {Err: domainlist.ErrCircularInclusion, Path: []string{"a", "a"}, Cycle: 0},
		"circular inclusion: top -> a -> [b -> c -> b]": {Err: domainlist.ErrCircularInclusion, Path: []string{"top", "a", "b", "c", "b"}, Cycle: 2},
		// 8 lists are not shortened.
		"list not found: l0 -> l1 -> l2 -> l3 -> l4 -> l5 -> l6 -> l7":                    {Err: domainlist.ErrListNotFound, Path: names(8), Cycle: -1},
		"list not found: l0 -> l1 -> l2 -> ...(2 lists)... -> l5 -> l6 -> l7 -> l8":       {Err: domainlist.ErrListNotFound, Path: names(9), Cycle: -1},
		"circular inclusion: [l0 -> l1 -> l2 -> ...(3 lists)... -> l6 -> l7 -> l8 -> l0]": {Err: domainlist.ErrCircularInclusion, Path: append(names(9), "l0"), Cycle: 0},
		// The circle starts among the omitted lists.
		"circular inclusion: l0 -> l1 -> l2 -> [...(3 lists)... -> l6 -> l7 -> l8 -> l4]": {Err: domainlist.ErrCircularInclusion, Path: append(names(9), "l4"), Cycle: 4},
		"circular inclusion: l0 -> l1 -> l2 -> ...(3 lists)... -> l6 -> [l7 -> l8 -> l7]": {Err: domainlist.ErrCircularInclusion, Path: append(names(9), "l7"), Cycle: 7},
	} {
		require.EqualError(t, err, message)
	}

	// The full path is got by errors.As.
	m := domainlist.NewMerger(setOf(t, map[string]string{"top": "include:a", "a": "include:b", "b": "include:a"}))
	_, err := m.Merge("top")
	require.ErrorIs(t, err, domainlist.ErrCircularInclusion)
	var mergeErr *domainlist.MergeError
	require.ErrorAs(t, err, &mergeErr)
	require.Equal(t, []string{"top", "a", "b", "a"}, mergeErr.Path)
	require.Equal(t, 1, mergeErr.Cycle)

	_, err = domainlist.NewMerger(&domainlist.FileLookuper{FS: fstest.MapFS{"top": {Data: []byte("include:bad")}, "bad": {Data: []byte("bad.com attr")}}}).Merge("top")
	require.EqualError(t, err, `lookup top -> bad: read bad: line 1: unknown field: "attr"`)
	require.ErrorAs(t, err, &mergeErr)
	require.Equal(t, []string{"top", "bad"}, mergeErr.Path)
	require.NotErrorIs(t, err, domainlist.ErrListNotFound)
}

// BenchmarkMergeAttrsDataFiles compares MergeAttrs with filtering the merged list, see BenchmarkMergeAllDataFiles.
func BenchmarkMergeAttrsDataFiles(b *testing.B) {
	dir := os.Getenv("DLC_DATA")
	if dir == "" {
		b.Skip("DLC_DATA is not set")
	}
	set := domainlist.NewDomainListSet()
	for list, err := range (&domainlist.FileLookuper{FS: os.DirFS(dir)}).All() {
		require.NoError(b, err)
		set.Add(list)
	}
	m := domainlist.NewMerger(set)
	const name = "geolocation-!cn"

	b.Run("filter", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			list, err := m.Merge(name)
			if err != nil {
				b.Fatal(err)
			}
			list.Filter([]string{"cn"}, nil)
		}
	})
	b.Run("attrs", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, err := m.MergeAttrs(name, "cn")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
