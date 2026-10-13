// Package rulex validates the values of domain rules, e.g. domain names, keywords and regexps,
// and converts internationalized domain names to punycode.
package rulex

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/imduaky/ruleset/pkg/internal/common"

	"golang.org/x/net/idna"
)

const (
	maxDomainLen = 253 // Maximum length of a domain name
	maxLabelLen  = 63  // Maximum length of a label of a domain name
)

var (
	ErrInvalidDomain  = errors.New("invalid domain")
	ErrInvalidKeyword = errors.New("invalid keyword")
	ErrInvalidRegexp  = errors.New("invalid regexp")
)

var domainChars = common.NewCharset("abcdefghijklmnopqrstuvwxyz0123456789.-")

// IsDomain reports whether s is a valid domain name, so that typos like
// "example..com" or "-example.com" would not become rules which never match.
// ASCII names, including punycode ones, must be lowercase. Internationalized names, e.g.
// "例子.com", must be valid for registration, where the runes mapped by ToASCII,
// e.g. uppercase or fullwidth ones, are not allowed either.
func IsDomain(s string) bool {
	if !isASCII(s) {
		ascii, err := idna.Registration.ToASCII(s)
		if err != nil {
			return false
		}
		s = ascii
	}
	return isASCIIDomain(s)
}

func isASCIIDomain(s string) bool {
	if len(s) > maxDomainLen || !domainChars.All(s) {
		return false
	}
	for label := range strings.SplitSeq(s, ".") {
		if label == "" || len(label) > maxLabelLen || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
	}
	return true
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// ToASCII converts the internationalized labels of the domain name to punycode, e.g. "例子.com"
// to "xn--fsqu00a.com", after mapping them for lookup by UTS #46, e.g. "MÜNCHEN.de" to
// "xn--mnchen-3ya.de" and "例子。com" to "xn--fsqu00a.com". ASCII names are returned as is,
// the result is not validated, see IsDomain.
func ToASCII(s string) (string, error) {
	if isASCII(s) {
		return s, nil
	}
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil {
		return "", fmt.Errorf("%w %q: %w", ErrInvalidDomain, s, err)
	}
	return ascii, nil
}

func IsKeyword(s string) bool { return domainChars.All(s) }

func ValidateDomain(s string) error {
	if !IsDomain(s) {
		return fmt.Errorf("%w: %q", ErrInvalidDomain, s)
	}
	return nil
}

func ValidateKeyword(s string) error {
	if !IsKeyword(s) {
		return fmt.Errorf("%w: %q", ErrInvalidKeyword, s)
	}
	return nil
}

func ValidateRegexp(s string) error {
	if s == "" {
		return fmt.Errorf("%w: %q", ErrInvalidRegexp, s)
	}

	// Use regexp.Compile instead of syntax.Parse: Compile performs both parsing
	// and compilation, so it catches all errors the real regexp engine would.
	// syntax.Parse only parses and may succeed even when compilation fails.
	_, err := regexp.Compile(s)
	if err != nil {
		return fmt.Errorf("%w %q: %w", ErrInvalidRegexp, s, err)
	}
	return nil
}
