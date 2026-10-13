package domainlist_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/imduaky/ruleset/pkg/domainlist"
	"github.com/imduaky/ruleset/pkg/internal/testcase"

	"github.com/stretchr/testify/require"
)

// TestText runs the cases in testdata/text, see testdata/text/sample.txtar.
func TestText(t *testing.T) {
	testcase.Run(t, "testdata/text/*.txtar", func(t *testing.T, c testcase.Case) {
		input := c.Get(t, "input")
		if message, ok := c.Lookup("error"); ok {
			var list domainlist.DomainList
			err := list.UnmarshalText([]byte(input))
			require.ErrorContains(t, err, "unmarshalText: "+strings.TrimSpace(message))
			_, err = domainlist.Read(strings.NewReader(input))
			require.ErrorContains(t, err, strings.TrimSpace(message))
			return
		}

		output := c.Get(t, "output")
		list := parseText(t, input)
		require.Equal(t, list, parseText(t, strings.ReplaceAll(input, "\n", "\r\n")), "CRLF")
		data, err := list.MarshalText()
		require.NoError(t, err)
		require.Equal(t, output, string(data))

		// AppendText appends to the prefix.
		data, err = list.AppendText([]byte("# head\n"))
		require.NoError(t, err)
		require.Equal(t, "# head\n"+output, string(data))

		// The output is canonical.
		decoded := parseText(t, output)
		require.Equal(t, list, decoded)
		again, err := decoded.MarshalText()
		require.NoError(t, err)
		require.Equal(t, output, string(again))

		read, err := domainlist.Read(strings.NewReader(input))
		require.NoError(t, err)
		require.Equal(t, list, *read)
		var buffer bytes.Buffer
		require.NoError(t, domainlist.Write(&buffer, *read))
		require.Equal(t, output, buffer.String())

		if message, ok := c.Lookup("invalid"); ok {
			require.ErrorContains(t, list.Validate(), strings.TrimSpace(message))
		} else {
			require.NoError(t, list.Validate())
		}
	})
}

// TestToASCII runs the cases in testdata/ascii.txtar.
func TestToASCII(t *testing.T) {
	testcase.Run(t, "testdata/ascii.txtar", func(t *testing.T, c testcase.Case) {
		input := parseText(t, c.Get(t, "input"))
		list := parseText(t, c.Get(t, "input"))
		err := list.ToASCII()
		if message, ok := c.Lookup("error"); ok {
			require.ErrorContains(t, err, strings.TrimSpace(message))
			// It is not modified on error.
			require.Equal(t, input, list)
			return
		}
		require.NoError(t, err)
		data, err := list.MarshalText()
		require.NoError(t, err)
		require.Equal(t, c.Get(t, "output"), string(data))

		// Each domain is converted the same, and converting again changes nothing.
		for i, d := range input.Domains {
			require.NoError(t, d.ToASCII())
			require.Equal(t, list.Domains[i], d)
		}
		again := list
		require.NoError(t, again.ToASCII())
		require.Equal(t, list, again)
	})
}

func parseText(t testing.TB, text string) domainlist.DomainList {
	t.Helper()
	var list domainlist.DomainList
	require.NoError(t, list.UnmarshalText([]byte(text)))
	return list
}

func TestEmpty(t *testing.T) {
	for _, text := range []string{"", "\n", "# comment only\n  \t\n"} {
		var list domainlist.DomainList
		require.NoError(t, list.UnmarshalText([]byte(text)), text)
		require.Equal(t, domainlist.DomainList{}, list, text)
	}
	data, err := domainlist.DomainList{}.MarshalText()
	require.NoError(t, err)
	require.Empty(t, data)

	var list domainlist.DomainList
	require.NoError(t, list.UnmarshalText([]byte("include:a\n")))
	require.Equal(t, domainlist.DomainList{Inclusions: []domainlist.Inclusion{{Name: "a"}}}, list)
}

func TestValidate(t *testing.T) {
	require.NoError(t, domainlist.DomainList{}.Validate())

	for message, list := range map[string]domainlist.DomainList{
		`domains[0]: unknown rule type: ""`:           {Domains: []domainlist.Domain{{Value: "example.com"}}},
		`domains[0]: invalid value: ""`:               {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeKeyword}}},
		`domains[0]: invalid domain: "Example.com"`:   {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeDomain, Value: "Example.com"}}},
		`domains[1]: invalid keyword: "Google"`:       {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeFull, Value: "a.com"}, {Type: domainlist.RuleTypeKeyword, Value: "Google"}}},
		`domains[0]: invalid regexp "a)"`:             {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeRegexp, Value: "a)"}}},
		`domains[0]: invalid attribute: "Ads"`:        {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeFull, Value: "a.com", Attrs: []string{"Ads"}}}},
		`inclusions[0]: invalid included list name`:   {Inclusions: []domainlist.Inclusion{{}}},
		`inclusions[0]: invalid ban attribute: "a b"`: {Inclusions: []domainlist.Inclusion{{Name: "a", BanAttrs: []string{"a b"}}}},
	} {
		require.ErrorContains(t, list.Validate(), message)
	}
}

func TestMarshalErrors(t *testing.T) {
	for message, list := range map[string]domainlist.DomainList{
		`domains[0]: unknown rule type: ""`:           {Domains: []domainlist.Domain{{Value: "example.com"}}},
		`domains[0]: unknown rule type: "include"`:    {Domains: []domainlist.Domain{{Type: domainlist.PrefixInclude, Value: "a"}}},
		`domains[0]: invalid value: ""`:               {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeRegexp}}},
		`domains[1]: invalid value: "a b"`:            {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeFull, Value: "a.com"}, {Type: domainlist.RuleTypeRegexp, Value: "a b"}}},
		`domains[0]: invalid value: "a#b"`:            {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeRegexp, Value: "a#b"}}},
		`domains[0]: invalid value: "a.com\n"`:        {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeDomain, Value: "a.com\n"}}},
		`domains[0]: invalid attribute: "Ads"`:        {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeFull, Value: "a.com", Attrs: []string{"Ads"}}}},
		`domains[0]: invalid affiliation: "a b"`:      {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeFull, Value: "a.com", Affiliations: []string{"a b"}}}},
		`inclusions[0]: invalid included list name`:   {Inclusions: []domainlist.Inclusion{{}}},
		`inclusions[0]: invalid must attribute: "-a"`: {Inclusions: []domainlist.Inclusion{{Name: "a", MustAttrs: []string{"-a"}}}},
		`inclusions[0]: invalid ban attribute: ""`:    {Inclusions: []domainlist.Inclusion{{Name: "a", BanAttrs: []string{""}}}},
	} {
		_, err := list.MarshalText()
		require.ErrorContains(t, err, message)

		// Nothing is appended on error.
		data, err := list.AppendText([]byte("head"))
		require.Error(t, err, message)
		require.Equal(t, "head", string(data), message)
	}

	// Values are not validated.
	data, err := domainlist.DomainList{Domains: []domainlist.Domain{
		{Type: domainlist.RuleTypeDomain, Value: "Example.com"},
		{Type: domainlist.RuleTypeRegexp, Value: "("},
	}}.MarshalText()
	require.NoError(t, err)
	require.Equal(t, "Example.com\nregexp:(\n", string(data))
}

func TestDomainText(t *testing.T) {
	var d domainlist.Domain
	require.NoError(t, d.UnmarshalText([]byte(" FULL:WWW.Example.com @cn @ads &Other ")))
	expected := domainlist.Domain{Type: domainlist.RuleTypeFull, Value: "www.example.com", Attrs: []string{"ads", "cn"}, Affiliations: []string{"other"}}
	require.Equal(t, expected, d)

	data, err := d.MarshalText()
	require.NoError(t, err)
	require.Equal(t, "full:www.example.com @ads @cn &other", string(data))
	require.Equal(t, string(data), d.String())

	require.NoError(t, d.UnmarshalText([]byte("example.com")))
	require.Equal(t, domainlist.Domain{Type: domainlist.RuleTypeDomain, Value: "example.com"}, d)
	require.Equal(t, "example.com", d.String())

	// The value is not validated.
	require.NoError(t, d.UnmarshalText([]byte("keyword:goo_gle")))
	require.ErrorContains(t, d.Validate(), `invalid keyword: "goo_gle"`)

	for _, text := range []string{"", "include:list", "example.com # comment", "example.com\nexample.org"} {
		require.Error(t, d.UnmarshalText([]byte(text)), text)
	}
	// String does not validate.
	require.Equal(t, "bad:Value", domainlist.Domain{Type: "bad", Value: "Value"}.String())
	_, err = domainlist.Domain{Type: "bad", Value: "Value"}.MarshalText()
	require.ErrorContains(t, err, `marshalText: unknown rule type: "bad"`)
}

func TestInclusionText(t *testing.T) {
	var inc domainlist.Inclusion
	require.NoError(t, inc.UnmarshalText([]byte("Include:List @-CN @ads")))
	expected := domainlist.Inclusion{Name: "list", MustAttrs: []string{"ads"}, BanAttrs: []string{"cn"}}
	require.Equal(t, expected, inc)

	data, err := inc.MarshalText()
	require.NoError(t, err)
	require.Equal(t, "include:list @ads @-cn", string(data))
	require.Equal(t, string(data), inc.String())

	err = inc.UnmarshalText([]byte("example.com"))
	require.ErrorContains(t, err, "not an inclusion")
	require.Error(t, inc.UnmarshalText([]byte("include:list &other")))

	require.Equal(t, "include:List", domainlist.Inclusion{Name: "List"}.String())
	_, err = domainlist.Inclusion{Name: "List"}.MarshalText()
	require.ErrorContains(t, err, `marshalText: invalid included list name: "List"`)
}

func TestReadWriteErrors(t *testing.T) {
	errRead := errors.New("read error")
	_, err := domainlist.Read(iotest.ErrReader(errRead))
	require.ErrorIs(t, err, errRead)

	var buffer bytes.Buffer
	require.Error(t, domainlist.Write(&buffer, domainlist.DomainList{Domains: []domainlist.Domain{{}}}))
	require.Zero(t, buffer.Len())
}

// TestDataFiles checks the data directory of v2fly/domain-list-community, e.g.
// DLC_DATA=domain-list-community/data go test -run TestDataFiles ./pkg/domainlist
func TestDataFiles(t *testing.T) {
	dir := os.Getenv("DLC_DATA")
	if dir == "" {
		t.Skip("DLC_DATA is not set")
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	var lists []domainlist.NamedDomainList
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		require.NoError(t, err)

		var list domainlist.DomainList
		require.NoError(t, list.UnmarshalText(content), path)
		require.Equal(t, countRules(string(content)), len(list.Inclusions)+len(list.Domains), path)

		data, err := list.MarshalText()
		require.NoError(t, err, path)
		var decoded domainlist.DomainList
		require.NoError(t, decoded.UnmarshalText(data), path)
		require.Equal(t, list, decoded, path)
		again, err := decoded.MarshalText()
		require.NoError(t, err, path)
		require.Equal(t, data, again, path)

		require.NoError(t, list.Validate(), path)
		lists = append(lists, domainlist.NamedDomainList{Name: entry.Name(), DomainList: list})
	}

	all, err := domainlist.NewMerger(domainlist.NewDomainListSet(lists...)).MergeAll()
	require.NoError(t, err)
	require.Equal(t, len(lists), all.Len())
	// The files are looked up lazily, and merged the same.
	fromFiles, err := domainlist.NewMerger(&domainlist.FileLookuper{FS: os.DirFS(dir)}).MergeAll()
	require.NoError(t, err)
	require.Equal(t, all, fromFiles)
	m := domainlist.NewMerger(domainlist.NewDomainListSet(lists...))
	for list, err := range all.All() {
		require.NoError(t, err)
		name := list.Name
		// MergeAttrs filters while merging, the same as filtering the merged list.
		for _, attrs := range [][]string{{"cn"}, {"-cn"}, {"!cn"}, {"ads"}, {"ads", "-cn"}} {
			filtered, err := m.MergeAttrs(name, attrs...)
			require.NoError(t, err)
			var must, ban []string
			for _, attr := range attrs {
				if b, ok := strings.CutPrefix(attr, "-"); ok {
					ban = append(ban, b)
				} else {
					must = append(must, attr)
				}
			}
			require.Equal(t, list.Filter(must, ban), filtered.Domains, "%s %v", name, attrs)
		}
		require.Empty(t, list.Inclusions, name)
		require.NoError(t, list.Validate(), name)
	}
}

// countRules counts the lines which are neither empty nor comment.
func countRules(text string) int {
	n := 0
	for line := range strings.Lines(text) {
		line, _, _ = strings.Cut(line, "#")
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func benchmarkList(n int) domainlist.DomainList {
	list := domainlist.DomainList{Inclusions: []domainlist.Inclusion{{Name: "other", BanAttrs: []string{"cn"}}}}
	for i := range n {
		d := domainlist.Domain{Type: domainlist.RuleTypeDomain, Value: "sub" + strconv.Itoa(i) + ".example.com"}
		if i%4 == 0 {
			d.Type, d.Attrs = domainlist.RuleTypeFull, []string{"ads"}
		}
		list.Domains = append(list.Domains, d)
	}
	return list
}

func BenchmarkMarshalText(b *testing.B) {
	list := benchmarkList(10000)
	b.ReportAllocs()
	for b.Loop() {
		_, err := list.MarshalText()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalText(b *testing.B) {
	data, err := benchmarkList(10000).MarshalText()
	require.NoError(b, err)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		var list domainlist.DomainList
		err := list.UnmarshalText(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
