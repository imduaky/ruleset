package domainlist

import (
	"encoding"
	"fmt"
	"slices"
	"strings"
)

var (
	_ encoding.TextAppender    = Domain{}
	_ encoding.TextMarshaler   = Domain{}
	_ encoding.TextUnmarshaler = (*Domain)(nil)
)

// Domain is a domain rule, e.g. `full:www.example.com @ads &other-list`.
type Domain struct {
	Type  RuleType
	Value string

	// Attrs are kept in the final lists, they are sorted and deduplicated when parsing.
	Attrs []string
	// Affiliations are the names of other lists which the rule is added to as well.
	Affiliations []string
}

func (d Domain) String() string {
	return string(d.append(make([]byte, 0, d.textLen())))
}

func (d Domain) AppendText(b []byte) ([]byte, error) {
	err := d.validate()
	if err != nil {
		return b, err
	}
	return d.append(b), nil
}

func (d Domain) MarshalText() ([]byte, error) {
	data, err := d.AppendText(make([]byte, 0, d.textLen()))
	if err != nil {
		return nil, fmt.Errorf("marshalText: %w", err)
	}
	return data, nil
}

func (d *Domain) UnmarshalText(data []byte) error {
	typ, rule := cutType(strings.TrimSpace(string(data)))
	dd, err := parseDomain(RuleType(typ), rule)
	if err != nil {
		return fmt.Errorf("unmarshalText: %w", err)
	}
	*d = dd
	return nil
}

// parseDomain parses the rule after the type, the same as the official.
func parseDomain(typ RuleType, rule string) (Domain, error) {
	value, rest := cutField(rule)
	if typ != RuleTypeRegexp {
		value = strings.ToLower(value)
	}
	d := Domain{Type: typ, Value: value}
	for field := range strings.FieldsSeq(rest) {
		switch field[:1] {
		case SymbolAttribute:
			d.Attrs = append(d.Attrs, strings.ToLower(field[1:]))
		case SymbolAffiliation:
			d.Affiliations = append(d.Affiliations, strings.ToLower(field[1:]))
		default:
			return Domain{}, fmt.Errorf("unknown field: %q", field)
		}
	}
	slices.Sort(d.Attrs)
	d.Attrs = slices.Compact(d.Attrs)

	err := d.validate()
	if err != nil {
		return Domain{}, err
	}
	return d, nil
}

func (d Domain) validate() error {
	err := validateValue(d.Type, d.Value)
	if err != nil {
		return err
	}
	err = validateAll(d.Attrs, isAttr, "attribute")
	if err != nil {
		return err
	}
	return validateAll(d.Affiliations, isListName, "affiliation")
}

// textLen returns the exact length written by append.
func (d Domain) textLen() int {
	n := len(d.Value)
	if d.Type != RuleTypeDomain {
		n += len(d.Type) + len(SymbolType)
	}
	return n + fieldsLen(SymbolAttribute, d.Attrs) + fieldsLen(SymbolAffiliation, d.Affiliations)
}

// append writes `type:value @attr &affiliation`, the type is omitted for RuleTypeDomain.
func (d Domain) append(b []byte) []byte {
	if d.Type != RuleTypeDomain {
		b = append(b, d.Type...)
		b = append(b, SymbolType...)
	}
	b = append(b, d.Value...)
	b = appendFields(b, SymbolAttribute, d.Attrs)
	return appendFields(b, SymbolAffiliation, d.Affiliations)
}
