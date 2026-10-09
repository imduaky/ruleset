// Modified from sing, Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>, GPL-3.0-or-later.
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/domain/matcher.go
// https://github.com/SagerNet/sing/blob/6f21f2425a959912c37d2ef43d61e2a663315dea/common/domain/adguard_matcher.go

package singdomain

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/imduaky/ruleset/pkg/internal/singvarbin"
)

const (
	prefixLabel = '\r'
	rootLabel   = '\n'
	suffixLabel = '\b'
)

func WriteMatcher(w singvarbin.Writer, domains, suffixes []string, legacy bool) error {
	keys := make([]string, 0, len(domains)+2*len(suffixes))
	seen := make(map[string]bool)
	for _, value := range suffixes {
		if value == "" {
			return fmt.Errorf("domain: empty domain suffix")
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		if value[0] == '.' {
			keys = append(keys, reverseDomain(string(prefixLabel)+value))
		} else if legacy {
			keys = append(keys, reverseDomain(value))
			suffix := "." + value
			if !seen[suffix] {
				seen[suffix] = true
				keys = append(keys, reverseDomain(string(prefixLabel)+suffix))
			}
		} else {
			keys = append(keys, reverseDomain(string(rootLabel)+value))
		}
	}
	for _, value := range domains {
		if value == "" {
			return fmt.Errorf("domain: empty domain")
		}
		if !seen[value] {
			seen[value] = true
			keys = append(keys, reverseDomain(value))
		}
	}
	slices.Sort(keys)
	return newSuccinctSet(slices.Compact(keys)).Write(w)
}

func ReadMatcher(r singvarbin.Reader) ([]string, []string, error) {
	ss, err := readSuccinctSet(r)
	if err != nil {
		return nil, nil, err
	}
	domains := make(map[string]bool)
	prefixes := make(map[string]bool)
	var suffixes []string
	for _, key := range ss.keys() {
		key = reverseDomain(key)
		if len(key) == 0 {
			domains[key] = true
			continue
		}
		switch key[0] {
		case prefixLabel:
			prefixes[key[1:]] = true
		case rootLabel:
			suffixes = append(suffixes, key[1:])
		default:
			domains[key] = true
		}
	}
	for value := range prefixes {
		if strings.HasPrefix(value, ".") && domains[value[1:]] {
			delete(domains, value[1:])
			suffixes = append(suffixes, value[1:])
		} else {
			suffixes = append(suffixes, value)
		}
	}
	var result []string
	for value := range domains {
		result = append(result, value)
	}
	slices.Sort(result)
	slices.Sort(suffixes)
	return result, suffixes, nil
}

func WriteAdGuardMatcher(w singvarbin.Writer, rules []string) error {
	keys := make([]string, 0, len(rules))
	for _, value := range rules {
		isSuffix, hasStart, hasEnd := false, false, false
		if strings.HasPrefix(value, "||") {
			value = value[2:]
			isSuffix = true
		} else if strings.HasPrefix(value, "|") {
			value = value[1:]
			hasStart = true
		}
		if strings.HasSuffix(value, "^") {
			value = value[:len(value)-1]
			hasEnd = true
		}
		if value == "" {
			return fmt.Errorf("domain: empty AdGuard rule")
		}
		if isSuffix {
			value = string(rootLabel) + value
		} else if !hasStart {
			value = string(prefixLabel) + value
		}
		if !hasEnd {
			value = strings.TrimSuffix(value, ".") + string(suffixLabel)
		}
		keys = append(keys, reverseDomain(value))
	}
	slices.Sort(keys)
	return newSuccinctSet(slices.Compact(keys)).Write(w)
}

func ReadAdGuardMatcher(r singvarbin.Reader) ([]string, error) {
	ss, err := readSuccinctSet(r)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, key := range ss.keys() {
		key = reverseDomain(key)
		if len(key) == 0 {
			return nil, fmt.Errorf("domain: empty AdGuard trie key")
		}
		prefix := "|"
		if key[0] == prefixLabel {
			prefix, key = "", key[1:]
		} else if key[0] == rootLabel {
			prefix, key = "||", key[1:]
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("domain: empty AdGuard trie key")
		}
		if key[len(key)-1] == suffixLabel {
			key = key[:len(key)-1]
		} else {
			key += "^"
		}
		result = append(result, prefix+key)
	}
	return result, nil
}

func reverseDomain(value string) string {
	data := make([]byte, len(value))
	for i := 0; i < len(value); {
		r, size := utf8.DecodeRuneInString(value[i:])
		i += size
		utf8.EncodeRune(data[len(value)-i:], r)
	}
	// data is allocated here and never modified after, so it is safe to share it.
	return unsafe.String(unsafe.SliceData(data), len(data))
}
