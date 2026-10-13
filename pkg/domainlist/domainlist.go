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

// DomainList is a single domain list file.
// the raw file acts like:
//
// include:cn
// full:example.com @example
// com
//
// Comments and the order between inclusions and domains are not kept,
// inclusions are written before domains.
type DomainList struct {
	Inclusions []Inclusion
	Domains    []Domain
}

func Read(r io.Reader) (*DomainList, error) {
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
	err := l.check(Domain.check)
	if err != nil {
		return b, err
	}
	return l.appendText(slices.Grow(b, l.textLen())), nil
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

func (l DomainList) Validate() error {
	return l.check(Domain.Validate)
}

// ToASCII converts the internationalized domain names to punycode, see Domain.ToASCII.
// l is not modified on error.
func (l *DomainList) ToASCII() error {
	type change struct {
		i     int
		value string
	}
	var changes []change
	for i, d := range l.Domains {
		value, err := d.asciiValue()
		if err != nil {
			return fmt.Errorf("domains[%d]: %w", i, err)
		}
		if value != d.Value {
			changes = append(changes, change{i: i, value: value})
		}
	}
	for _, c := range changes {
		l.Domains[c.i].Value = c.value
	}
	return nil
}

// Copy returns a deep Copy of the list.
func (l DomainList) Copy() DomainList {
	list := DomainList{Inclusions: slices.Clone(l.Inclusions), Domains: slices.Clone(l.Domains)}
	for i := range list.Inclusions {
		inc := &list.Inclusions[i]
		inc.MustAttrs, inc.BanAttrs = slices.Clone(inc.MustAttrs), slices.Clone(inc.BanAttrs)
	}
	for i := range list.Domains {
		d := &list.Domains[i]
		d.Attrs, d.Affiliations = slices.Clone(d.Attrs), slices.Clone(d.Affiliations)
	}
	return list
}

// Filter returns the domains with all of must and none of ban attributes,
// the same as `include:list @must @-ban`. Inclusions are not merged.
func (l DomainList) Filter(must, ban []string) []Domain {
	f := newAttrFilter(must, ban)
	var domains []Domain
	for _, d := range l.Domains {
		if f.match(d.Attrs) {
			domains = append(domains, d)
		}
	}
	return domains
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

func (l DomainList) check(checkDomain func(Domain) error) error {
	for i, inc := range l.Inclusions {
		err := inc.check()
		if err != nil {
			return fmt.Errorf("inclusions[%d]: %w", i, err)
		}
	}
	for i, d := range l.Domains {
		err := checkDomain(d)
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

func (l DomainList) appendText(b []byte) []byte {
	for _, inc := range l.Inclusions {
		b = append(inc.appendText(b), '\n')
	}
	for _, d := range l.Domains {
		b = append(d.appendText(b), '\n')
	}
	return b
}

func cutType(line string) (typ, rule string) {
	typ, rule, ok := strings.Cut(line, SymbolType)
	if !ok {
		return string(RuleTypeDomain), line
	}
	return strings.ToLower(typ), rule
}

func cutField(s string) (field, rest string) {
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	i := strings.IndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i:]
}

func fieldsLen(symbol string, values []string) int {
	n := len(values) * (len(" ") + len(symbol))
	for _, value := range values {
		n += len(value)
	}
	return n
}

func appendFields(b []byte, symbol string, values []string) []byte {
	for _, value := range values {
		b = append(b, ' ')
		b = append(b, symbol...)
		b = append(b, value...)
	}
	return b
}
