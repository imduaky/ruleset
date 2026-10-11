package domainlist

import (
	"encoding"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
)

var (
	_ encoding.TextAppender    = (*DomainList)(nil)
	_ encoding.TextMarshaler   = (*DomainList)(nil)
	_ encoding.TextUnmarshaler = (*DomainList)(nil)
)

// DomainList is a list file under the data directory of v2fly/domain-list-community.
// Comments and the order between inclusions and domains are not kept,
// inclusions are written before domains.
type DomainList struct {
	Inclusions []Inclusion
	Domains    []Domain
}

func Read(r io.Reader) (*DomainList, error) {
	// Values are substrings of the text, strings.Builder avoids copying it again.
	var text strings.Builder
	_, err := io.Copy(&text, r)
	if err != nil {
		return nil, err
	}
	list, err := parse(text.String())
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func Write(w io.Writer, list DomainList) error {
	data, err := list.AppendText(nil)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (l DomainList) AppendText(b []byte) ([]byte, error) {
	err := l.validate()
	if err != nil {
		return b, err
	}
	return l.append(slices.Grow(b, l.textLen())), nil
}

func (l DomainList) MarshalText() ([]byte, error) {
	data, err := l.AppendText(nil)
	if err != nil {
		return nil, fmt.Errorf("marshalText: %w", err)
	}
	return data, nil
}

func (l *DomainList) UnmarshalText(data []byte) error {
	list, err := parse(string(data))
	if err != nil {
		return fmt.Errorf("unmarshalText: %w", err)
	}
	*l = list
	return nil
}

func parse(text string) (DomainList, error) {
	list := DomainList{Domains: make([]Domain, 0, strings.Count(text, "\n")+1)}
	lineNo := 0
	for line := range strings.Lines(text) {
		lineNo++
		line, _, _ = strings.Cut(line, SymbolComment)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		typ, rule := cutType(line)
		if typ == PrefixInclude {
			inc, err := parseInclusion(rule)
			if err != nil {
				return DomainList{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			list.Inclusions = append(list.Inclusions, inc)
		} else {
			d, err := parseDomain(RuleType(typ), rule)
			if err != nil {
				return DomainList{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			list.Domains = append(list.Domains, d)
		}
	}
	if len(list.Domains) == 0 {
		list.Domains = nil
	}
	return list, nil
}

func (l DomainList) validate() error {
	for i, inc := range l.Inclusions {
		err := inc.validate()
		if err != nil {
			return fmt.Errorf("inclusions[%d]: %w", i, err)
		}
	}
	for i, d := range l.Domains {
		err := d.validate()
		if err != nil {
			return fmt.Errorf("domains[%d]: %w", i, err)
		}
	}
	return nil
}

func (l DomainList) textLen() int {
	n := len(l.Inclusions) + len(l.Domains) // LF
	for _, inc := range l.Inclusions {
		n += inc.textLen()
	}
	for _, d := range l.Domains {
		n += d.textLen()
	}
	return n
}

// append writes one rule per line.
func (l DomainList) append(b []byte) []byte {
	for _, inc := range l.Inclusions {
		b = append(inc.append(b), '\n')
	}
	for _, d := range l.Domains {
		b = append(d.append(b), '\n')
	}
	return b
}

// cutType cuts the lowercased type before the first SymbolType, the type is RuleTypeDomain if omitted.
func cutType(line string) (typ, rule string) {
	typ, rule, ok := strings.Cut(line, SymbolType)
	if !ok {
		return string(RuleTypeDomain), line
	}
	return strings.ToLower(typ), rule
}

// cutField cuts the first field of s, the same as strings.Fields.
func cutField(s string) (field, rest string) {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	i := strings.IndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i:]
}

// fieldsLen returns the length written by appendFields.
func fieldsLen(symbol string, values []string) int {
	n := len(values) * (len(" ") + len(symbol))
	for _, value := range values {
		n += len(value)
	}
	return n
}

// appendFields writes ` <symbol><value>` for each value.
func appendFields(b []byte, symbol string, values []string) []byte {
	for _, value := range values {
		b = append(b, ' ')
		b = append(b, symbol...)
		b = append(b, value...)
	}
	return b
}
