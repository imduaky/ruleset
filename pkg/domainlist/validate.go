package domainlist

import (
	"fmt"
	"regexp/syntax"
	"strings"
	"unicode"
)

type charset [256]bool

func newCharset(chars string) *charset {
	var set charset
	for i := range len(chars) {
		set[chars[i]] = true
	}
	return &set
}

// all reports whether s is not empty and only consists of chars in the set.
func (set *charset) all(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if !set[s[i]] {
			return false
		}
	}
	return true
}

const lowerDigits = "abcdefghijklmnopqrstuvwxyz0123456789"

// Values are checked after being lowercased, while the official checks list names after being uppercased.
var (
	domainChars   = newCharset(lowerDigits + ".-")
	attrChars     = newCharset(lowerDigits + "!")
	listNameChars = newCharset(lowerDigits + "!-")
)

// isDomainName reports whether s is a valid domain name, so that typos like
// "example..com" or "-example.com" would not become rules which never match.
func isDomainName(s string) bool {
	if len(s) > maxDomainLen || !domainChars.all(s) {
		return false
	}
	for label := range strings.SplitSeq(s, ".") {
		if label == "" || len(label) > maxLabelLen || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
	}
	return true
}

func isAttr(s string) bool { return attrChars.all(s) }

func isListName(s string) bool { return listNameChars.all(s) }

func validateValue(typ RuleType, value string) error {
	switch typ {
	case RuleTypeDomain, RuleTypeFull:
		if !isDomainName(value) {
			return fmt.Errorf("invalid domain: %q", value)
		}
	case RuleTypeKeyword:
		if !domainChars.all(value) {
			return fmt.Errorf("invalid keyword: %q", value)
		}
	case RuleTypeRegexp:
		// The value ends at a space or a comment, so it could not be read back.
		if value == "" || strings.ContainsFunc(value, unicode.IsSpace) || strings.Contains(value, SymbolComment) {
			return fmt.Errorf("invalid regexp: %q", value)
		}
		// regexp.Compile only fails when parsing.
		_, err := syntax.Parse(value, syntax.Perl)
		if err != nil {
			return fmt.Errorf("invalid regexp %q: %w", value, err)
		}
	default:
		return fmt.Errorf("unknown rule type: %q", typ)
	}
	return nil
}

func validateAll(values []string, valid func(string) bool, name string) error {
	for _, value := range values {
		if !valid(value) {
			return fmt.Errorf("invalid %s: %q", name, value)
		}
	}
	return nil
}
