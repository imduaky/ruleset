package domainlist

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDomainName(t *testing.T) {
	label := strings.Repeat("a", maxLabelLen)
	longest := label + "." + label + "." + label + "." + strings.Repeat("b", maxDomainLen-3*(maxLabelLen+1))
	require.Len(t, longest, maxDomainLen)

	for value, valid := range map[string]bool{
		"example.com":       true,
		"localhost":         true,
		"a-b.c-d.example":   true,
		"xn--fiqs8s":        true,
		"0.1.example":       true,
		label + ".com":      true,
		longest:             true,
		"":                  false,
		".":                 false,
		".example.com":      false,
		"example.com.":      false,
		"example..com":      false,
		"-example.com":      false,
		"example-.com":      false,
		"Example.com":       false,
		"exa_mple.com":      false,
		"例子.com":            false,
		label + "a.com":     false,
		longest[1:] + ".b":  false,
		"example.com @attr": false,
	} {
		require.Equal(t, valid, isDomainName(value), value)
	}
}

func TestCharsets(t *testing.T) {
	for value, valid := range map[string]bool{"google": true, "-.": true, "go-ogle.": true, "": false, "Google": false, "go_ogle": false} {
		require.Equal(t, valid, domainChars.all(value), "keyword %q", value)
	}
	for value, valid := range map[string]bool{"ads": true, "!cn": true, "a1": true, "": false, "-cn": false, "Ads": false, "a.b": false} {
		require.Equal(t, valid, isAttr(value), "attribute %q", value)
	}
	for value, valid := range map[string]bool{"category-ads-all": true, "geolocation-!cn": true, "": false, "Google": false, "a_b": false, "a.b": false} {
		require.Equal(t, valid, isListName(value), "list name %q", value)
	}
}

func TestValidateValue(t *testing.T) {
	require.NoError(t, validateValue(RuleTypeRegexp, `^a:b\.(c|d)$`))
	require.NoError(t, validateValue(RuleTypeRegexp, `@`))
	for _, value := range []string{"", "(", "a b", "a\tb", "a#b", `\`} {
		require.Error(t, validateValue(RuleTypeRegexp, value), value)
	}
	for _, typ := range []RuleType{"", "Domain", PrefixInclude} {
		require.ErrorContains(t, validateValue(typ, "example.com"), "unknown rule type", typ)
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
		require.Len(t, d.append(nil), d.textLen(), d.String())
	}

	// MarshalText allocates exactly once.
	b := make([]byte, 0, list.textLen())
	data := list.append(b)
	require.Len(t, data, cap(b))
	require.Same(t, &b[:1][0], &data[0])
}
