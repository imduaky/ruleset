package rulex_test

import (
	"testing"

	"github.com/imduaky/ruleset/pkg/rulex"

	"github.com/stretchr/testify/require"
)

func TestValidateDomain(t *testing.T) {
	require.NoError(t, rulex.ValidateDomain("www.example.com"))
	for _, value := range []string{"", "example..com", "-example.com", "Example.com", "exa_mple.com", "example.com #"} {
		err := rulex.ValidateDomain(value)
		require.ErrorIs(t, err, rulex.ErrInvalidDomain, value)
		require.EqualError(t, err, `invalid domain: "`+value+`"`)
	}
}

func TestValidateDomainIDN(t *testing.T) {
	require.NoError(t, rulex.ValidateDomain("例子.com"))
	require.NoError(t, rulex.ValidateDomain("xn--fsqu00a.com"))
	err := rulex.ValidateDomain("例子。com")
	require.ErrorIs(t, err, rulex.ErrInvalidDomain)
	require.EqualError(t, err, `invalid domain: "例子。com"`)
}

func TestToASCII(t *testing.T) {
	for value, expected := range map[string]string{
		"例子.com":             "xn--fsqu00a.com",
		"münchen.de":         "xn--mnchen-3ya.de",
		"MÜNCHEN.DE":         "xn--mnchen-3ya.de",
		"faß.de":             "xn--fa-hia.de",
		"日本.jp":              "xn--wgv71a.jp",
		"ドメイン名例.jp":          "xn--eckwd4c7cu47r2wf.jp",
		"مثال.إختبار":        "xn--mgbh0fb.xn--kgbechtv",
		"例子。com":             "xn--fsqu00a.com",
		"ｅｘａｍｐｌｅ.com":        "example.com",
		"xn--fsqu00a.例子.com": "xn--fsqu00a.xn--fsqu00a.com",
		// ASCII names are kept as is.
		"example.com":  "example.com",
		"Example.COM":  "Example.COM",
		"example..com": "example..com",
		"":             "",
	} {
		ascii, err := rulex.ToASCII(value)
		require.NoError(t, err, value)
		require.Equal(t, expected, ascii, value)
		if expected != "example..com" && expected != "Example.COM" && expected != "" {
			require.True(t, rulex.IsDomain(ascii), ascii)
		}
	}

	for value, message := range map[string]string{
		"-例子.com":         `invalid domain "-例子.com": idna: invalid label "-例子"`,
		"exa_mple.例子.com": `invalid domain "exa_mple.例子.com": idna: disallowed rune U+005F`,
	} {
		ascii, err := rulex.ToASCII(value)
		require.ErrorIs(t, err, rulex.ErrInvalidDomain, value)
		require.EqualError(t, err, message)
		require.Empty(t, ascii)
	}
}

func TestValidateKeyword(t *testing.T) {
	for value, valid := range map[string]bool{
		"google":   true,
		"go-ogle.": true,
		"-.":       true, // not a domain name, but a part of it
		"":         false,
		"Google":   false,
		"go_ogle":  false,
		"go ogle":  false,
	} {
		require.Equal(t, valid, rulex.IsKeyword(value), value)
		err := rulex.ValidateKeyword(value)
		if valid {
			require.NoError(t, err, value)
		} else {
			require.ErrorIs(t, err, rulex.ErrInvalidKeyword, value)
		}
	}
}

func TestValidateRegexp(t *testing.T) {
	for _, value := range []string{`^a:b\.(c|d)$`, `@`, `a b`, `a#b`, `(?i)example`} {
		require.NoError(t, rulex.ValidateRegexp(value), value)
	}
	for value, message := range map[string]string{
		"":     `invalid regexp: ""`,
		"(":    `invalid regexp "(": error parsing regexp: missing closing ): ` + "`(`",
		`\`:    `invalid regexp "\\": error parsing regexp: trailing backslash at end of expression: ` + "``",
		`a{2,`: ``, // a literal in Go
	} {
		err := rulex.ValidateRegexp(value)
		if message == "" {
			require.NoError(t, err, value)
			continue
		}
		require.ErrorIs(t, err, rulex.ErrInvalidRegexp, value)
		require.EqualError(t, err, message)
	}
}

func BenchmarkIsDomain(b *testing.B) {
	for _, value := range []string{"sub.www.example.com", "sub.www.例子.com", "sub.www.xn--fsqu00a.com"} {
		b.Run(value, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !rulex.IsDomain(value) {
					b.Fatal("invalid")
				}
			}
		})
	}
}
