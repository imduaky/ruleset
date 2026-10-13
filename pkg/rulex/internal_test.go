package rulex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/idna"
)

func TestIsDomain(t *testing.T) {
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
		label + "a.com":     false,
		longest[1:] + ".b":  false,
		"example.com @attr": false,
	} {
		require.Equal(t, valid, IsDomain(value), value)
	}
}

func TestIsDomainIDN(t *testing.T) {
	// The longest label of distinct runes, and the shortest one too long after being converted.
	label, tooLong := "", ""
	for r := rune(0x4e00); ; r += 7 {
		ascii, err := idna.Lookup.ToASCII(label + string(r))
		if err != nil || len(ascii) > maxLabelLen {
			tooLong = label + string(r)
			break
		}
		label += string(r)
	}
	ascii, err := idna.Lookup.ToASCII(tooLong)
	require.NoError(t, err)
	require.Greater(t, len(ascii), maxLabelLen)
	for value, valid := range map[string]bool{
		"例子.com":             true,
		"münchen.de":         true,
		"faß.de":             true, // not mapped to "fass.de"
		"日本.jp":              true,
		"مثال.إختبار":        true,
		"xn--fsqu00a.com":    true,
		"xn--fsqu00a.例子.com": true,
		label + ".com":       true,
		tooLong + ".com":     false, // too long after being converted
		"MÜNCHEN.de":         false, // uppercase
		"例子.COM":             false,
		"ｅｘａｍｐｌｅ.com":        false, // fullwidth
		"例子。com":             false, // ideographic full stop
		"例子..com":            false,
		"-例子.com":            false,
		"例子-.com":            false,
		"exa_mple.例子.com":    false,
		"例子 .com":            false,
		"xn--zz.com":         true,  // ASCII names are not decoded
		"xn--zz.例子.com":      false, // invalid punycode
		"xn--fsqu00a.COM":    false,
		"\xff.com":           false, // invalid UTF-8
	} {
		require.Equal(t, valid, IsDomain(value), value)
	}
}

func TestIsASCII(t *testing.T) {
	for value, ascii := range map[string]bool{"": true, "example.com": true, "\x7f": true, "例子.com": false, "\x80": false, "ü": false} {
		require.Equal(t, ascii, isASCII(value), value)
	}
}
