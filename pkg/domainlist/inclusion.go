package domainlist

import (
	"encoding"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	_ encoding.TextAppender    = (*Inclusion)(nil)
	_ encoding.TextMarshaler   = (*Inclusion)(nil)
	_ encoding.TextUnmarshaler = (*Inclusion)(nil)
)

type Inclusion struct {
	Name string

	MustAttrs []string
	BanAttrs  []string
}

func (inc Inclusion) Equal(ii Inclusion) bool {
	return inc.Name == ii.Name &&
		slices.Equal(inc.MustAttrs, ii.MustAttrs) &&
		slices.Equal(inc.BanAttrs, ii.BanAttrs)
}

func (inc Inclusion) String() string {
	return string(inc.append(make([]byte, 0, inc.textLen())))
}

func (inc Inclusion) AppendText(b []byte) ([]byte, error) {
	err := inc.check()
	if err != nil {
		return b, err
	}
	return inc.append(b), nil
}

func (inc Inclusion) MarshalText() ([]byte, error) {
	data, err := inc.AppendText(make([]byte, 0, inc.textLen()))
	if err != nil {
		return nil, fmt.Errorf("marshalText: %w", err)
	}
	return data, nil
}

func (inc *Inclusion) UnmarshalText(data []byte) error {
	typ, rule := cutType(strings.TrimSpace(string(data)))
	if typ != PrefixInclude {
		return fmt.Errorf("unmarshalText: not an inclusion: %q", data)
	}
	ii, err := parseInclusion(rule)
	if err != nil {
		return fmt.Errorf("unmarshalText: %w", err)
	}
	*inc = ii
	return nil
}

func parseInclusion(rule string) (Inclusion, error) {
	name, rest := cutField(rule)
	inc := Inclusion{Name: strings.ToLower(name)}
	for field := range strings.FieldsSeq(rest) {
		switch field[:1] {
		case SymbolAttribute:
			attr := strings.ToLower(field[1:])
			if ban, ok := strings.CutPrefix(attr, SymbolBan); ok {
				inc.BanAttrs = append(inc.BanAttrs, ban)
			} else {
				inc.MustAttrs = append(inc.MustAttrs, attr)
			}
		case SymbolAffiliation:
			return Inclusion{}, errors.New("affiliation is not allowed for inclusion")
		default:
			return Inclusion{}, fmt.Errorf("unknown field: %q", field)
		}
	}

	err := inc.check()
	if err != nil {
		return Inclusion{}, err
	}
	return inc, nil
}

func (inc Inclusion) check() error {
	if !isListName(inc.Name) {
		return fmt.Errorf("invalid included list name: %q", inc.Name)
	}
	err := validateAll(inc.MustAttrs, isAttr, "must attribute")
	if err != nil {
		return err
	}
	return validateAll(inc.BanAttrs, isAttr, "ban attribute")
}

func (inc Inclusion) textLen() int {
	return len(PrefixInclude) + len(SymbolType) + len(inc.Name) +
		fieldsLen(SymbolAttribute, inc.MustAttrs) + fieldsLen(SymbolAttribute+SymbolBan, inc.BanAttrs)
}

func (inc Inclusion) append(b []byte) []byte {
	b = append(b, PrefixInclude+SymbolType...)
	b = append(b, inc.Name...)
	b = appendFields(b, SymbolAttribute, inc.MustAttrs)
	return appendFields(b, SymbolAttribute+SymbolBan, inc.BanAttrs)
}
