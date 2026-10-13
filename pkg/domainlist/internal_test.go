package domainlist

import (
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCharsets(t *testing.T) {
	for value, valid := range map[string]bool{"ads": true, "!cn": true, "a1": true, "": false, "-cn": false, "Ads": false, "a.b": false} {
		require.Equal(t, valid, isAttr(value), "attribute %q", value)
	}
	for value, valid := range map[string]bool{"category-ads-all": true, "geolocation-!cn": true, "": false, "Google": false, "a_b": false, "a.b": false} {
		require.Equal(t, valid, isListName(value), "list name %q", value)
	}
}

func TestIsValue(t *testing.T) {
	for value, valid := range map[string]bool{"example.com": true, "Example..com": true, `^a:b$`: true, "@a": true, "": false, "a b": false, "a\tb": false, "a\u3000b": false, "a#b": false} {
		require.Equal(t, valid, isValue(value), "value %q", value)
	}
}

func TestValidateValue(t *testing.T) {
	require.NoError(t, validateValue(RuleTypeRegexp, `^a:b\.(c|d)$`))
	require.NoError(t, validateValue(RuleTypeRegexp, `@`))
	for _, value := range []string{"", "(", `\`} {
		require.Error(t, validateValue(RuleTypeRegexp, value), value)
	}
	require.NoError(t, validateValue(RuleTypeKeyword, "google"))
	require.Error(t, validateValue(RuleTypeKeyword, "go_ogle"))
	require.NoError(t, validateValue(RuleTypeFull, "example.com"))
	require.Error(t, validateValue(RuleTypeDomain, "example..com"))
	for _, typ := range []RuleType{"", "Domain", PrefixInclude} {
		require.ErrorContains(t, validateValue(typ, "example.com"), "unknown rule type", typ)
		require.False(t, isRuleType(typ), typ)
	}
}

func TestCutType(t *testing.T) {
	for line, expected := range map[string][2]string{
		"example.com":       {"domain", "example.com"},
		"example.com @a":    {"domain", "example.com @a"},
		"FULL:example.com":  {"full", "example.com"},
		"regexp:^a:b$":      {"regexp", "^a:b$"},
		":example.com":      {"", "example.com"},
		"include: list @a ": {"include", " list @a "},
	} {
		typ, rule := cutType(line)
		require.Equal(t, expected, [2]string{typ, rule}, line)
	}
}

func TestCutField(t *testing.T) {
	for s, expected := range map[string][2]string{
		"":              {"", ""},
		"   ":           {"", ""},
		"a":             {"a", ""},
		" \ta  @b @c ":  {"a", "  @b @c "},
		"a @b":          {"a", " @b"},
		"example.com @": {"example.com", " @"},
	} {
		field, rest := cutField(s)
		require.Equal(t, expected, [2]string{field, rest}, s)
	}
}

func TestTextLen(t *testing.T) {
	list := DomainList{
		Inclusions: []Inclusion{
			{Name: "a"},
			{Name: "list", MustAttrs: []string{"ads", "!cn"}, BanAttrs: []string{"cn", "x"}},
		},
		Domains: []Domain{
			{Type: RuleTypeDomain, Value: "example.com"},
			{Type: RuleTypeDomain, Value: "example.com", Attrs: []string{"a", "bc"}, Affiliations: []string{"list"}},
			{Type: RuleTypeFull, Value: "www.example.com", Affiliations: []string{"a", "b"}},
			{Type: RuleTypeKeyword, Value: "google"},
			{Type: RuleTypeRegexp, Value: `^a\.b$`, Attrs: []string{"cn"}},
		},
	}
	for _, inc := range list.Inclusions {
		require.Len(t, inc.append(nil), inc.textLen(), inc.String())
	}
	for _, d := range list.Domains {
		require.Len(t, d.appendText(nil), d.textLen(), d.String())
	}

	// MarshalText allocates exactly once.
	b := make([]byte, 0, list.textLen())
	data := list.appendText(b)
	require.Len(t, data, cap(b))
	require.Same(t, &b[:1][0], &data[0])
}

func TestHasAttrs(t *testing.T) {
	for _, c := range []struct {
		attrs, must, ban []string
		has              bool
	}{
		{nil, nil, nil, true},
		{nil, nil, []string{"cn"}, true},
		{nil, []string{"ads"}, nil, false},
		{[]string{"ads", "cn"}, nil, nil, true},
		{[]string{"ads", "cn"}, []string{"cn", "ads"}, nil, true},
		{[]string{"ads", "cn"}, []string{"ads"}, []string{"!cn"}, true},
		{[]string{"ads", "cn"}, []string{"ads"}, []string{"cn"}, false},
		{[]string{"ads"}, []string{"ads", "cn"}, nil, false},
	} {
		require.Equal(t, c.has, matchAttrs(c.attrs, c.must, c.ban), "%v %v %v", c.attrs, c.must, c.ban)
	}
}

func TestKeyOf(t *testing.T) {
	d := Domain{Type: RuleTypeFull, Value: "example.com", Attrs: []string{"a", "b"}}
	other := d
	other.Affiliations = []string{"list"}
	require.Equal(t, keyOf(d), keyOf(other))
	for _, dd := range []Domain{
		{Type: RuleTypeDomain, Value: "example.com", Attrs: []string{"a", "b"}},
		{Type: RuleTypeFull, Value: "example.org", Attrs: []string{"a", "b"}},
		{Type: RuleTypeFull, Value: "example.com", Attrs: []string{"a"}},
		{Type: RuleTypeFull, Value: "example.com"},
	} {
		require.NotEqual(t, keyOf(d), keyOf(dd), dd.String())
	}

	// Keys of domains with less than 2 attrs are free.
	d.Attrs = d.Attrs[:1]
	require.Zero(t, testing.AllocsPerRun(10, func() { keyOf(d) }))
}

// TestMergerReuse checks that a merged list is cached and reused, and the looked up lists are not modified.
func TestMergerReuse(t *testing.T) {
	named := func(name string, list DomainList) NamedDomainList {
		return NamedDomainList{Name: name, DomainList: list}
	}
	c := DomainList{Inclusions: []Inclusion{{Name: "d"}}, Domains: []Domain{{Type: RuleTypeDomain, Value: "c.com"}}}
	set := NewDomainListSet(
		named("a", DomainList{Inclusions: []Inclusion{{Name: "c"}}}),
		named("b", DomainList{Inclusions: []Inclusion{{Name: "c", MustAttrs: []string{"x"}}}}),
		named("c", c),
		named("d", DomainList{Domains: []Domain{{Type: RuleTypeDomain, Value: "d.com", Attrs: []string{"x"}}}}),
	)
	m := NewMerger(set)
	_, err := m.Merge("a")
	require.NoError(t, err)
	merged := m.merged["c"]
	require.Empty(t, merged.Inclusions)
	require.Len(t, merged.Domains, 2)
	list, err := set.Lookup("c")
	require.NoError(t, err)
	require.Equal(t, c, list)
	cached, err := m.lookuper.Lookup("c")
	require.NoError(t, err)
	require.Equal(t, c, cached)

	_, err = m.Merge("b")
	require.NoError(t, err)
	require.Same(t, &merged.Domains[0], &m.merged["c"].Domains[0], "c is not merged again")
}

func TestAttrFilter(t *testing.T) {
	many := func(prefix string, n int) []string {
		s := make([]string, n)
		for i := range s {
			s[i] = prefix + strconv.Itoa(i)
		}
		return s
	}
	for _, c := range []struct {
		attrs, must, ban []string
		match            bool
	}{
		{nil, nil, nil, true},
		{nil, nil, []string{"cn"}, true},
		{nil, []string{"ads"}, nil, false},
		{[]string{"ads", "cn"}, []string{"cn", "ads"}, nil, true},
		{[]string{"ads", "cn"}, []string{"ads"}, []string{"cn"}, false},
		{[]string{"ads", "ads"}, []string{"ads", "cn"}, nil, false},
		// Many attributes are looked up in maps.
		{many("a", 20), many("a", 10), many("b", 10), true},
		{many("a", 20), many("a", 21), nil, false},
		{append(many("a", 20), "b9"), many("a", 10), many("b", 10), false},
		{append(many("a", 5), "a1", "a1", "a1"), many("a", 9), nil, false},
		{many("a", 9), append(many("a", 9), "a0", "a8"), nil, true},
		{many("a", 64), many("a", 64), nil, true},
		{many("a", 63), many("a", 64), nil, false},
		// More must attributes than the bits are looked up in slices.
		{many("a", 70), many("a", 70), nil, true},
		{many("a", 69), many("a", 70), nil, false},
	} {
		f := newAttrFilter(c.must, c.ban)
		require.Equal(t, c.match, f.match(c.attrs), "%v %v %v", c.attrs, c.must, c.ban)
		require.Equal(t, matchAttrs(c.attrs, c.must, c.ban), f.match(c.attrs), "%v %v %v", c.attrs, c.must, c.ban)
		require.Equal(t, len(c.must)+len(c.ban) >= minMapAttrs && len(slices.Compact(slices.Sorted(slices.Values(c.must)))) <= maxMustBits, f.mustBits != nil)
	}
}

func BenchmarkAttrFilter(b *testing.B) {
	many := func(prefix string, n int) []string {
		s := make([]string, n)
		for i := range s {
			s[i] = prefix + strconv.Itoa(i)
		}
		return s
	}
	for _, c := range []struct {
		name             string
		attrs, must, ban []string
	}{
		{"none", []string{"ads"}, nil, nil},
		{"1must-1ban", []string{"ads", "x"}, []string{"ads"}, []string{"cn"}},
		{"10must-10ban", many("a", 20), many("a", 10), many("b", 10)},
		{"30must-30ban", many("a", 50), many("a", 30), many("b", 30)},
	} {
		f := newAttrFilter(c.must, c.ban)
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				f.match(c.attrs)
			}
		})
	}
}

func TestMergeContextReset(t *testing.T) {
	c := new(mergeContext)
	list := DomainList{Domains: []Domain{{Type: RuleTypeDomain, Value: "a.com"}}}
	small, large := make(map[domainKey]struct{}), make(map[domainKey]struct{})
	for i := range maxPooledEntries + 1 {
		key := domainKey{value: strconv.Itoa(i)}
		if i < 10 {
			small[key] = struct{}{}
		}
		large[key] = struct{}{}
	}
	c.push("a", list)
	c.push("b", list)
	c.release(small)
	c.release(large)
	require.Empty(t, small)

	// A failed merge leaves the frames on the stack.
	stack := c.stack[:cap(c.stack)]
	c.reset()
	require.Empty(t, c.stack)
	require.Empty(t, c.onStack)
	for _, f := range stack {
		require.Zero(t, f, "the lists are not referenced")
	}
	// The small map is kept to be reused, while the large one is dropped.
	require.Len(t, c.seens, 1)
	m := c.newSeen(0)
	m[domainKey{}] = struct{}{}
	require.Len(t, small, 1, "small is reused")

	// A deep stack is dropped.
	for i := range maxPooledFrames + 1 {
		c.push(strconv.Itoa(i), list)
	}
	for range maxPooledFrames + 1 {
		c.pop()
	}
	c.reset()
	require.Nil(t, c.stack)
	require.Nil(t, c.onStack)

	// The pooled contexts are reset.
	c = getContext()
	c.push("a", list)
	putContext(c)
	require.Empty(t, c.stack)
	require.Empty(t, c.onStack)
}
