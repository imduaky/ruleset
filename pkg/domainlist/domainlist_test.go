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

	"github.com/stretchr/testify/require"
)

const sampleText = `# comment
include:Other-List
include:ads @ADS @-cn @!cn # trailing comment

Example.COM @b @a @b &Affiliated
domain:example.org
FULL:www.example.com @ads
keyword:Google
regexp:^odd[1-7]\.example\.org(\.[a-z]{2})?$
	  sub.example.net  	@cn	`

const sampleCanonical = `include:other-list
include:ads @ads @!cn @-cn
example.com @a @b &affiliated
example.org
full:www.example.com @ads
keyword:google
regexp:^odd[1-7]\.example\.org(\.[a-z]{2})?$
sub.example.net @cn
`

func sampleList() domainlist.DomainList {
	return domainlist.DomainList{
		Inclusions: []domainlist.Inclusion{
			{Name: "other-list"},
			{Name: "ads", MustAttrs: []string{"ads", "!cn"}, BanAttrs: []string{"cn"}},
		},
		Domains: []domainlist.Domain{
			{Type: domainlist.RuleTypeDomain, Value: "example.com", Attrs: []string{"a", "b"}, Affiliations: []string{"affiliated"}},
			{Type: domainlist.RuleTypeDomain, Value: "example.org"},
			{Type: domainlist.RuleTypeFull, Value: "www.example.com", Attrs: []string{"ads"}},
			{Type: domainlist.RuleTypeKeyword, Value: "google"},
			{Type: domainlist.RuleTypeRegexp, Value: `^odd[1-7]\.example\.org(\.[a-z]{2})?$`},
			{Type: domainlist.RuleTypeDomain, Value: "sub.example.net", Attrs: []string{"cn"}},
		},
	}
}

func TestUnmarshal(t *testing.T) {
	for name, text := range map[string]string{
		"LF":   sampleText,
		"CRLF": strings.ReplaceAll(sampleText, "\n", "\r\n"),
	} {
		var list domainlist.DomainList
		require.NoError(t, list.UnmarshalText([]byte(text)), name)
		require.Equal(t, sampleList(), list, name)
	}
}

func TestMarshal(t *testing.T) {
	data, err := sampleList().MarshalText()
	require.NoError(t, err)
	require.Equal(t, sampleCanonical, string(data))

	prefix := []byte("# head\n")
	data, err = sampleList().AppendText(prefix)
	require.NoError(t, err)
	require.Equal(t, string(prefix)+sampleCanonical, string(data))
}

func TestRoundTrip(t *testing.T) {
	data, err := sampleList().MarshalText()
	require.NoError(t, err)
	var decoded domainlist.DomainList
	require.NoError(t, decoded.UnmarshalText(data))
	require.Equal(t, sampleList(), decoded)

	again, err := decoded.MarshalText()
	require.NoError(t, err)
	require.Equal(t, data, again)
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

func TestUnmarshalNormalize(t *testing.T) {
	var list domainlist.DomainList
	require.NoError(t, list.UnmarshalText([]byte(`domain: Example.com   @Z @a @z`)))
	require.Equal(t, []domainlist.Domain{{Type: domainlist.RuleTypeDomain, Value: "example.com", Attrs: []string{"a", "z"}}}, list.Domains)

	// The case of a regexp is kept.
	require.NoError(t, list.UnmarshalText([]byte(`REGEXP:^A\.Example$`)))
	require.Equal(t, []domainlist.Domain{{Type: domainlist.RuleTypeRegexp, Value: `^A\.Example$`}}, list.Domains)

	// Only the first SymbolType separates the type.
	require.NoError(t, list.UnmarshalText([]byte(`regexp:^a:b$`)))
	require.Equal(t, `^a:b$`, list.Domains[0].Value)
}

func TestUnmarshalErrors(t *testing.T) {
	for line, message := range map[string]string{
		"full:":                `invalid domain: ""`,
		":example.com":         `unknown rule type: ""`,
		"unknown:example.com":  `unknown rule type: "unknown"`,
		"example..com":         `invalid domain: "example..com"`,
		"-example.com":         `invalid domain`,
		"example.com-":         `invalid domain`,
		"exa_mple.com":         `invalid domain`,
		"keyword:goo_gle":      `invalid keyword: "goo_gle"`,
		"regexp:(":             `invalid regexp "("`,
		"example.com @":        `invalid attribute: ""`,
		"example.com @a-b":     `invalid attribute: "a-b"`,
		"example.com &":        `invalid affiliation: ""`,
		"example.com &a_b":     `invalid affiliation: "a_b"`,
		"example.com attr":     `unknown field: "attr"`,
		"include:":             `invalid included list name: ""`,
		"include:a_b":          `invalid included list name: "a_b"`,
		"include:a &b":         `affiliation is not allowed for inclusion`,
		"include:a @":          `invalid must attribute: ""`,
		"include:a @-":         `invalid ban attribute: ""`,
		"include:a @-a-b":      `invalid ban attribute: "a-b"`,
		"include:a other-list": `unknown field: "other-list"`,
	} {
		var list domainlist.DomainList
		err := list.UnmarshalText([]byte("example.com\n# comment\n" + line))
		require.ErrorContains(t, err, "line 3: "+message, line)
	}
}

func TestMarshalErrors(t *testing.T) {
	for message, list := range map[string]domainlist.DomainList{
		`domains[0]: unknown rule type: ""`:           {Domains: []domainlist.Domain{{Value: "example.com"}}},
		`domains[0]: unknown rule type: "include"`:    {Domains: []domainlist.Domain{{Type: domainlist.PrefixInclude, Value: "a"}}},
		`domains[0]: invalid domain: "Example.com"`:   {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeDomain, Value: "Example.com"}}},
		`domains[1]: invalid keyword: "Google"`:       {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeFull, Value: "a.com"}, {Type: domainlist.RuleTypeKeyword, Value: "Google"}}},
		`domains[0]: invalid regexp: ""`:              {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeRegexp}}},
		`domains[0]: invalid regexp: "a b"`:           {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeRegexp, Value: "a b"}}},
		`domains[0]: invalid regexp: "a#b"`:           {Domains: []domainlist.Domain{{Type: domainlist.RuleTypeRegexp, Value: "a#b"}}},
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

func TestReadWrite(t *testing.T) {
	list, err := domainlist.Read(strings.NewReader(sampleText))
	require.NoError(t, err)
	require.Equal(t, sampleList(), *list)

	var buffer bytes.Buffer
	require.NoError(t, domainlist.Write(&buffer, *list))
	require.Equal(t, sampleCanonical, buffer.String())

	errRead := errors.New("read error")
	_, err = domainlist.Read(iotest.ErrReader(errRead))
	require.ErrorIs(t, err, errRead)

	_, err = domainlist.Read(strings.NewReader("example..com"))
	require.ErrorContains(t, err, "line 1: invalid domain")

	buffer.Reset()
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
